package analytics

import (
	"GURLS-Backend/internal/repository"
	"GURLS-Backend/pkg/useragent"
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ClickData represents analytics data to be processed
type ClickData struct {
	Alias     string
	IPAddress *string
	UserAgent *string
	Referer   *string
	ClickedAt *time.Time
}

// ProcessorConfig holds configuration for the analytics processor
type ProcessorConfig struct {
	WorkerCount      int           // Number of worker goroutines
	BufferSize       int           // Size of the job queue buffer
	RetryAttempts    int           // Number of retry attempts for failed jobs
	RetryDelay       time.Duration // Base delay between retries
	ShutdownTimeout  time.Duration // Time to wait for graceful shutdown
	MaxBatchSize     int           // Maximum number of items to process in a batch
	BatchTimeout     time.Duration // Maximum time to wait before processing a batch
}

// DefaultConfig returns sensible default configuration
func DefaultConfig() ProcessorConfig {
	return ProcessorConfig{
		WorkerCount:     3,
		BufferSize:      1000,
		RetryAttempts:   3,
		RetryDelay:      time.Second,
		ShutdownTimeout: 30 * time.Second,
		MaxBatchSize:    10,
		BatchTimeout:    5 * time.Second,
	}
}

// Processor handles asynchronous analytics processing with reliability guarantees
type Processor struct {
	config   ProcessorConfig
	storage  repository.Storage
	log      *zap.Logger
	jobQueue chan *ClickData
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	started  bool
	mu       sync.RWMutex
}

// NewProcessor creates a new analytics processor
func NewProcessor(storage repository.Storage, log *zap.Logger, config ProcessorConfig) *Processor {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Processor{
		config:   config,
		storage:  storage,
		log:      log,
		jobQueue: make(chan *ClickData, config.BufferSize),
		ctx:      ctx,
		cancel:   cancel,
		started:  false,
	}
}

// Start begins processing analytics data
func (p *Processor) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		return fmt.Errorf("processor already started")
	}

	p.log.Info("starting analytics processor",
		zap.Int("workers", p.config.WorkerCount),
		zap.Int("buffer_size", p.config.BufferSize),
		zap.Int("retry_attempts", p.config.RetryAttempts),
	)

	// Start worker goroutines
	for i := 0; i < p.config.WorkerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	p.started = true
	return nil
}

// Stop gracefully shuts down the processor
func (p *Processor) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return fmt.Errorf("processor not started")
	}

	p.log.Info("stopping analytics processor")

	// Signal all workers to stop
	p.cancel()

	// Close the job queue to prevent new jobs
	close(p.jobQueue)

	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.log.Info("analytics processor stopped gracefully")
	case <-time.After(p.config.ShutdownTimeout):
		p.log.Warn("analytics processor shutdown timeout reached")
		return fmt.Errorf("shutdown timeout reached")
	}

	p.started = false
	return nil
}

// SubmitClick submits a click for asynchronous processing
func (p *Processor) SubmitClick(clickData *ClickData) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.started {
		return fmt.Errorf("processor not started")
	}

	select {
	case p.jobQueue <- clickData:
		p.log.Debug("click data submitted for processing", zap.String("alias", clickData.Alias))
		return nil
	case <-p.ctx.Done():
		return fmt.Errorf("processor is shutting down")
	default:
		// Queue is full, this is a critical situation
		p.log.Error("analytics queue is full, dropping click data", 
			zap.String("alias", clickData.Alias),
			zap.Int("queue_size", len(p.jobQueue)),
		)
		return fmt.Errorf("analytics queue is full")
	}
}

// worker processes analytics data with retry logic
func (p *Processor) worker(workerID int) {
	defer p.wg.Done()

	log := p.log.With(zap.Int("worker_id", workerID))
	log.Info("analytics worker started")

	for {
		select {
		case clickData := <-p.jobQueue:
			if clickData == nil {
				// Channel closed, worker should exit
				log.Info("analytics worker stopped")
				return
			}
			
			p.processClickWithRetry(log, clickData)

		case <-p.ctx.Done():
			log.Info("analytics worker received shutdown signal")
			return
		}
	}
}

// processClickWithRetry processes a single click with retry logic
func (p *Processor) processClickWithRetry(log *zap.Logger, clickData *ClickData) {
	var lastErr error

	for attempt := 1; attempt <= p.config.RetryAttempts; attempt++ {
		// Create a context with timeout for each attempt
		ctx, cancel := context.WithTimeout(p.ctx, 30*time.Second)
		
		err := p.processClick(ctx, log, clickData)
		cancel()

		if err == nil {
			// Success!
			if attempt > 1 {
				log.Info("click processing succeeded after retry",
					zap.String("alias", clickData.Alias),
					zap.Int("attempt", attempt),
				)
			}
			return
		}

		lastErr = err
		log.Warn("click processing failed",
			zap.String("alias", clickData.Alias),
			zap.Int("attempt", attempt),
			zap.Int("max_attempts", p.config.RetryAttempts),
			zap.Error(err),
		)

		// Don't retry on last attempt
		if attempt == p.config.RetryAttempts {
			break
		}

		// Exponential backoff delay
		delay := p.config.RetryDelay * time.Duration(1<<(attempt-1))
		
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-p.ctx.Done():
			log.Info("worker shutdown during retry delay")
			return
		}
	}

	// All retries failed
	log.Error("click processing failed after all retries",
		zap.String("alias", clickData.Alias),
		zap.Int("attempts", p.config.RetryAttempts),
		zap.Error(lastErr),
	)

	// TODO: Consider sending to dead letter queue or persistent storage
	// for manual investigation/retry later
}

// processClick processes a single click data entry
func (p *Processor) processClick(ctx context.Context, log *zap.Logger, clickData *ClickData) error {
	// Parse user agent to determine device type
	deviceType := "unknown"
	if clickData.UserAgent != nil {
		parser := useragent.GetGlobalParser()
		if parser != nil {
			deviceInfo := parser.ParseUserAgent(*clickData.UserAgent)
			deviceType = deviceInfo.DeviceType
			
			log.Debug("processed User-Agent",
				zap.String("device_type", deviceInfo.DeviceType),
				zap.String("browser", deviceInfo.Browser),
				zap.String("os", deviceInfo.OS),
				zap.String("alias", clickData.Alias),
			)
		} else {
			// Fallback to simple detection if parser not available
			ua := *clickData.UserAgent
			if containsIgnoreCase(ua, "Mobile") || containsIgnoreCase(ua, "Android") || containsIgnoreCase(ua, "iPhone") {
				deviceType = "mobile"
			} else if containsIgnoreCase(ua, "Tablet") || containsIgnoreCase(ua, "iPad") {
				deviceType = "tablet"
			} else if containsIgnoreCase(ua, "bot") || containsIgnoreCase(ua, "Bot") || containsIgnoreCase(ua, "Spider") {
				deviceType = "bot"
			} else {
				deviceType = "desktop"
			}
		}
	}

	// Record the click with advanced analytics
	err := p.storage.RecordClickAdvanced(
		ctx,
		clickData.Alias,
		deviceType,
		clickData.IPAddress,
		clickData.UserAgent,
		clickData.Referer,
		clickData.ClickedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to record click: %w", err)
	}

	log.Debug("click recorded successfully",
		zap.String("alias", clickData.Alias),
		zap.String("device_type", deviceType),
	)

	return nil
}

// GetStats returns processor statistics
func (p *Processor) GetStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"started":         p.started,
		"queue_length":    len(p.jobQueue),
		"queue_capacity":  cap(p.jobQueue),
		"worker_count":    p.config.WorkerCount,
		"retry_attempts":  p.config.RetryAttempts,
	}
}

// Helper functions

// containsIgnoreCase performs case-insensitive substring search
func containsIgnoreCase(str, substr string) bool {
	if str == "" || substr == "" {
		return false
	}
	
	strLower := toLower(str)
	substrLower := toLower(substr)
	
	for i := 0; i <= len(strLower)-len(substrLower); i++ {
		if strLower[i:i+len(substrLower)] == substrLower {
			return true
		}
	}
	return false
}

// toLower converts string to lowercase
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			result[i] = s[i] + ('a' - 'A')
		} else {
			result[i] = s[i]
		}
	}
	return string(result)
}