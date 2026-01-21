// Example demonstrating detailed timeout errors for FNS API
// Solves the original "context deadline exceeded" problem with minimal information
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	httpclient "github.com/rurick/http-client"
)

// NalogRuAuthRequest structure for authorization request in FNS API
type NalogRuAuthRequest struct {
	Inn      string `json:"inn"`
	Password string `json:"password"`
	DeviceOS string `json:"deviceOS"`
	DeviceId string `json:"deviceId"`
}

func main() {
	fmt.Println("=== Demonstration of Detailed Timeout Errors ===")

	// 1. Demonstrate problematic configuration (as it was)
	fmt.Println("1. Problematic configuration (as it was):")
	demonstrateProblematicConfig()

	fmt.Println("\n" + strings.Repeat("=", 70) + "\n")

	// 2. Demonstrate improved configuration (as it became)
	fmt.Println("2. Improved configuration (as it became):")
	demonstrateImprovedConfig()

	fmt.Println("\n" + strings.Repeat("=", 70) + "\n")

	// 3. Demonstrate handling of non-timeout errors
	fmt.Println("3. Handling other error types:")
	demonstrateNonTimeoutErrors()
}

// demonstrateProblematicConfig shows behavior with problematic configuration
func demonstrateProblematicConfig() {
	// Configuration with short timeouts (as it was before)
	config := httpclient.Config{
		Timeout:       5 * time.Second, // Too short for FNS API
		PerTryTimeout: 2 * time.Second, // Too short
		RetryEnabled:  false,           // Retry disabled
	}

	client := httpclient.New(config, "refuel-receipts-old")
	defer client.Close()

	// Create context with short timeout to simulate problem
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Prepare request
	authRequest := NalogRuAuthRequest{
		Inn:      "1234567890",
		Password: "password",
		DeviceOS: "iOS",
		DeviceId: "device123",
	}

	jsonBody, _ := json.Marshal(authRequest)

	// Make request to "slow" endpoint (simulating FNS API)
	resp, err := client.Post(ctx, "https://httpbin.org/delay/10", bytes.NewReader(jsonBody),
		httpclient.WithContentType("application/json"),
		httpclient.WithIdempotencyKey("auth-request-12345"),
	)

	if err != nil {
		// Demonstrate detailed error
		fmt.Printf("❌ Detailed error:\n%s\n", err.Error())

		// Check if this is a detailed timeout error
		var timeoutErr *httpclient.TimeoutError
		if errors.As(err, &timeoutErr) {
			fmt.Printf("\n🔍 Error analysis:\n")
			fmt.Printf("  • Method: %s\n", timeoutErr.Method)
			fmt.Printf("  • URL: %s\n", timeoutErr.URL)
			fmt.Printf("  • Host: %s\n", timeoutErr.Host)
			fmt.Printf("  • Attempt: %d of %d\n", timeoutErr.Attempt, timeoutErr.MaxAttempts)
			fmt.Printf("  • Overall timeout: %v\n", timeoutErr.Timeout)
			fmt.Printf("  • Per-try timeout: %v\n", timeoutErr.PerTryTimeout)
			fmt.Printf("  • Execution time: %v\n", timeoutErr.Elapsed)
			fmt.Printf("  • Timeout type: %s\n", timeoutErr.TimeoutType)
			fmt.Printf("  • Retry enabled: %t\n", timeoutErr.RetryEnabled)

			fmt.Printf("\n💡 Fix suggestions:\n")
			for i, suggestion := range timeoutErr.Suggestions {
				fmt.Printf("  %d. %s\n", i+1, suggestion)
			}
		}
		return
	}

	if resp != nil {
		resp.Body.Close()
		fmt.Printf("✅ Unexpected success: %s\n", resp.Status)
	}
}

// demonstrateImprovedConfig shows behavior with improved configuration
func demonstrateImprovedConfig() {
	// Improved configuration for working with FNS API
	config := httpclient.Config{
		Timeout:       60 * time.Second, // Increased overall timeout
		PerTryTimeout: 20 * time.Second, // Increased per-try timeout
		RetryEnabled:  true,             // Retry enabled
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts:       4, // 4 attempts
			BaseDelay:         500 * time.Millisecond,
			MaxDelay:          15 * time.Second,
			Jitter:            0.3, // 30% jitter
			RespectRetryAfter: true,
			// Additional statuses for retry
			RetryStatusCodes: []int{408, 429, 500, 502, 503, 504, 520, 521, 522, 524},
		},
		TracingEnabled: true,
	}

	client := httpclient.New(config, "refuel-receipts-improved")
	defer client.Close()

	ctx := context.Background()

	authRequest := NalogRuAuthRequest{
		Inn:      "1234567890",
		Password: "password",
		DeviceOS: "iOS",
		DeviceId: "device123",
	}

	jsonBody, _ := json.Marshal(authRequest)

	fmt.Println("Attempting request with improved configuration...")

	// Make request to fast endpoint to demonstrate success
	resp, err := client.Post(ctx, "https://httpbin.org/delay/1", bytes.NewReader(jsonBody),
		httpclient.WithContentType("application/json"),
		httpclient.WithIdempotencyKey("auth-request-improved-12345"),
	)

	if err != nil {
		fmt.Printf("❌ Error (even with improved configuration):\n%s\n", err.Error())

		var timeoutErr *httpclient.TimeoutError
		if errors.As(err, &timeoutErr) {
			fmt.Printf("\n💡 Suggestions:\n")
			for i, suggestion := range timeoutErr.Suggestions {
				fmt.Printf("  %d. %s\n", i+1, suggestion)
			}
		}
		return
	}

	if resp != nil {
		defer resp.Body.Close()
		fmt.Printf("✅ Successful request: %s\n", resp.Status)
		fmt.Printf("📊 Configuration worked:\n")
		fmt.Printf("  • Overall timeout: %v\n", config.Timeout)
		fmt.Printf("  • Per-try timeout: %v\n", config.PerTryTimeout)
		fmt.Printf("  • Max attempts: %d\n", config.RetryConfig.MaxAttempts)
		fmt.Printf("  • Retry enabled: %t\n", config.RetryEnabled)
	}
}

// demonstrateNonTimeoutErrors demonstrates that non-timeout errors are not changed
func demonstrateNonTimeoutErrors() {
	config := httpclient.Config{
		Timeout:       30 * time.Second,
		PerTryTimeout: 10 * time.Second,
		RetryEnabled:  true,
		RetryConfig: httpclient.RetryConfig{
			MaxAttempts: 3,
		},
	}

	client := httpclient.New(config, "error-demo")
	defer client.Close()

	ctx := context.Background()

	fmt.Println("Demonstrating various error types:")

	// 1. DNS error
	fmt.Println("\n1. DNS error:")
	_, err := client.Get(ctx, "https://nonexistent-domain-12345.com/api")
	if err != nil {
		var timeoutErr *httpclient.TimeoutError
		isTimeoutErr := errors.As(err, &timeoutErr)
		fmt.Printf("   Error: %s\n", err.Error())
		fmt.Printf("   Is TimeoutError? %t\n", isTimeoutErr)
	}

	// 2. Connection refused
	fmt.Println("\n2. Connection refused:")
	_, err = client.Get(ctx, "http://127.0.0.1:99999/api")
	if err != nil {
		var timeoutErr *httpclient.TimeoutError
		isTimeoutErr := errors.As(err, &timeoutErr)
		fmt.Printf("   Error: %s\n", err.Error())
		fmt.Printf("   Is TimeoutError? %t\n", isTimeoutErr)
	}

	// 3. HTTP status error
	fmt.Println("\n3. HTTP status error:")
	resp, err := client.Get(ctx, "https://httpbin.org/status/500")
	if err != nil {
		var timeoutErr *httpclient.TimeoutError
		isTimeoutErr := errors.As(err, &timeoutErr)
		fmt.Printf("   Error: %s\n", err.Error())
		fmt.Printf("   Is TimeoutError? %t\n", isTimeoutErr)
	} else if resp != nil {
		defer resp.Body.Close()
		fmt.Printf("   HTTP status: %s (not an error, but a response)\n", resp.Status)
		fmt.Printf("   This is not handled as TimeoutError\n")
	}

	fmt.Printf("\n✅ As you can see, only real timeout errors are enhanced.\n")
	fmt.Printf("   All other errors remain unchanged.\n")
}

// Helper function for demonstrating programmatic error handling
func handleError(err error, operation string) {
	if err == nil {
		return
	}

	// Check if this is a detailed timeout error
	var timeoutErr *httpclient.TimeoutError
	if errors.As(err, &timeoutErr) {
		log.Printf("❌ Timeout during %s:", operation)
		log.Printf("   URL: %s", timeoutErr.URL)
		log.Printf("   Attempt: %d/%d", timeoutErr.Attempt, timeoutErr.MaxAttempts)
		log.Printf("   Execution time: %v", timeoutErr.Elapsed)
		log.Printf("   Type: %s", timeoutErr.TimeoutType)

		// Programmatically handle different timeout types
		switch timeoutErr.TimeoutType {
		case "overall":
			log.Printf("   → Recommendation: increase overall timeout from %v", timeoutErr.Timeout)
		case "per-try":
			log.Printf("   → Recommendation: increase per-try timeout from %v", timeoutErr.PerTryTimeout)
		case "context":
			log.Printf("   → Recommendation: check context settings in calling code")
		}

		return
	}

	// Handle other error types as usual
	log.Printf("❌ Error during %s: %v", operation, err)
}
