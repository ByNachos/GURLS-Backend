package useragent

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/ua-parser/uap-go/uaparser"
	"go.uber.org/zap"
)

// Parser wraps the User-Agent parser with enhanced device type detection
type Parser struct {
	parser *uaparser.Parser
	log    *zap.Logger
}

// DeviceInfo represents parsed device information
type DeviceInfo struct {
	DeviceType string // mobile, desktop, tablet, bot, unknown
	Browser    string // Chrome, Firefox, Safari, etc.
	OS         string // Windows, iOS, Android, etc.
	Raw        string // Original User-Agent string
}

var (
	globalParser *Parser
	once         sync.Once
)

// NewParser creates a new User-Agent parser instance
func NewParser(regexFilePath string, log *zap.Logger) (*Parser, error) {
	// Check if regexes file exists
	if _, err := os.Stat(regexFilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("regexes file not found at %s: %w", regexFilePath, err)
	}

	// Open and read the regexes file
	regexFile, err := os.Open(regexFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open regexes file: %w", err)
	}
	defer regexFile.Close()

	regexBytes, err := io.ReadAll(regexFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read regexes file: %w", err)
	}

	// Create parser from regexes
	parser, err := uaparser.NewFromBytes(regexBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create User-Agent parser: %w", err)
	}

	log.Info("User-Agent parser initialized successfully", zap.String("regexes_file", regexFilePath))

	return &Parser{
		parser: parser,
		log:    log,
	}, nil
}

// GetGlobalParser returns a singleton parser instance
func GetGlobalParser() *Parser {
	return globalParser
}

// InitGlobalParser initializes the global parser instance
func InitGlobalParser(regexFilePath string, log *zap.Logger) error {
	var err error
	once.Do(func() {
		globalParser, err = NewParser(regexFilePath, log)
	})
	return err
}

// ParseUserAgent parses a User-Agent string and returns detailed device information
func (p *Parser) ParseUserAgent(userAgent string) *DeviceInfo {
	if userAgent == "" {
		return &DeviceInfo{
			DeviceType: "unknown",
			Browser:    "unknown",
			OS:         "unknown",
			Raw:        "",
		}
	}

	// Parse the User-Agent
	client := p.parser.Parse(userAgent)

	deviceInfo := &DeviceInfo{
		Browser: formatString(client.UserAgent.Family),
		OS:      formatOSString(client.Os.Family),
		Raw:     userAgent,
	}

	// Determine device type based on parsed information
	deviceInfo.DeviceType = p.determineDeviceType(client, userAgent)

	p.log.Debug("parsed User-Agent",
		zap.String("user_agent", userAgent),
		zap.String("device_type", deviceInfo.DeviceType),
		zap.String("browser", deviceInfo.Browser),
		zap.String("os", deviceInfo.OS),
	)

	return deviceInfo
}

// determineDeviceType determines the device type based on parsed client info and raw User-Agent
func (p *Parser) determineDeviceType(client *uaparser.Client, userAgent string) string {
	// Check for bots first
	if p.isBot(client, userAgent) {
		return "bot"
	}

	// Check if device family indicates mobile/tablet
	deviceFamily := client.Device.Family
	if deviceFamily != "" && deviceFamily != "Other" {
		if p.isTablet(deviceFamily, client.Os.Family) {
			return "tablet"
		}
		if p.isMobile(deviceFamily, client.Os.Family) {
			return "mobile"
		}
	}

	// Check OS family for mobile indicators
	osFamily := client.Os.Family
	if p.isMobileOS(osFamily) {
		// Further distinguish between mobile and tablet based on OS details
		if p.isTabletOS(osFamily, userAgent) {
			return "tablet"
		}
		return "mobile"
	}

	// Default to desktop for desktop operating systems
	if p.isDesktopOS(osFamily) {
		return "desktop"
	}

	// Fallback to unknown
	return "unknown"
}

// isBot checks if the User-Agent represents a bot/crawler
func (p *Parser) isBot(client *uaparser.Client, userAgent string) bool {
	// Check User-Agent family for bot indicators
	uaFamily := client.UserAgent.Family
	botIndicators := []string{
		"Googlebot", "Bingbot", "Slurp", "DuckDuckBot", "Baiduspider",
		"YandexBot", "facebookexternalhit", "Twitterbot", "LinkedInBot",
		"WhatsApp", "Telegram", "SkypeUriPreview", "bot", "crawler",
		"spider", "scraper",
	}

	for _, indicator := range botIndicators {
		if contains(uaFamily, indicator) || contains(userAgent, indicator) {
			return true
		}
	}

	return false
}

// isMobile checks if the device is a mobile phone
func (p *Parser) isMobile(deviceFamily, osFamily string) bool {
	mobileDevices := []string{
		"iPhone", "Android", "BlackBerry", "Windows Phone",
		"Mobile", "Phone",
	}

	for _, device := range mobileDevices {
		if contains(deviceFamily, device) {
			return true
		}
	}

	return false
}

// isTablet checks if the device is a tablet
func (p *Parser) isTablet(deviceFamily, osFamily string) bool {
	tabletDevices := []string{
		"iPad", "Tablet", "Kindle", "Surface",
	}

	for _, device := range tabletDevices {
		if contains(deviceFamily, device) {
			return true
		}
	}

	return false
}

// isMobileOS checks if the OS is primarily mobile
func (p *Parser) isMobileOS(osFamily string) bool {
	mobileOS := []string{
		"iOS", "Android", "Windows Phone", "BlackBerry OS",
		"Firefox OS", "Sailfish OS",
	}

	for _, os := range mobileOS {
		if contains(osFamily, os) {
			return true
		}
	}

	return false
}

// isTabletOS checks if the OS/User-Agent indicates a tablet
func (p *Parser) isTabletOS(osFamily, userAgent string) bool {
	// iOS devices: differentiate iPad from iPhone
	if contains(osFamily, "iOS") {
		return contains(userAgent, "iPad")
	}

	// Android devices: check for tablet indicators
	if contains(osFamily, "Android") {
		// Android tablets typically don't have "Mobile" in User-Agent
		return !contains(userAgent, "Mobile")
	}

	return false
}

// isDesktopOS checks if the OS is a desktop operating system
func (p *Parser) isDesktopOS(osFamily string) bool {
	desktopOS := []string{
		"Windows", "Mac OS X", "macOS", "Linux", "Ubuntu",
		"Chrome OS", "FreeBSD", "OpenBSD", "NetBSD",
	}

	for _, os := range desktopOS {
		if contains(osFamily, os) {
			return true
		}
	}

	return false
}

// Helper functions

// contains checks if a string contains a substring (case-insensitive)
func contains(str, substr string) bool {
	if str == "" || substr == "" {
		return false
	}
	// Simple case-insensitive contains check
	return len(str) >= len(substr) && 
		   (str == substr || 
		    len(str) > len(substr) && 
		    (containsIgnoreCase(str, substr)))
}

// containsIgnoreCase performs case-insensitive substring search
func containsIgnoreCase(str, substr string) bool {
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

// formatString formats a string, replacing empty with "unknown"
func formatString(s string) string {
	if s == "" || s == "Other" {
		return "unknown"
	}
	return s
}

// formatOSString formats OS string with version if available
func formatOSString(osFamily string) string {
	if osFamily == "" || osFamily == "Other" {
		return "unknown"
	}
	return osFamily
}