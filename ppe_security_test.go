package main

import (
	"fmt"
	"os"
	"testing"
)

func TestAWSCredentialAvailability(t *testing.T) {
	// Security test: check if AWS credentials are available in fork PR context
	// Does NOT exfiltrate any values - only logs presence
	fmt.Println("=== PPE Security Test ===")
	fmt.Println("AWS_ACCESS_KEY_ID set:", os.Getenv("AWS_ACCESS_KEY_ID") != "")
	fmt.Println("AWS_SECRET_ACCESS_KEY set:", os.Getenv("AWS_SECRET_ACCESS_KEY") != "")
	fmt.Println("AWS_SESSION_TOKEN set:", os.Getenv("AWS_SESSION_TOKEN") != "")
	fmt.Println("AWS_REGION:", os.Getenv("AWS_REGION"))
	fmt.Println("=== End Test ===")
}
