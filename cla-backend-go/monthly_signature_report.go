// Copyright The Linux Foundation.
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const (
	regionDefault = "us-east-1"
	profileName   = "lfproduct-prod"
	tableName     = "cla-prod-signatures"
)

// SignatureRecord represents the DynamoDB signature record structure
type SignatureRecord struct {
	SignatureID             string `dynamodbav:"signature_id"`
	DateCreated             string `dynamodbav:"date_created"`
	ApproxDateCreated       string `dynamodbav:"approx_date_created"`
	SignatureType           string `dynamodbav:"signature_type"`
	SigtypeSignedApprovedID string `dynamodbav:"sigtype_signed_approved_id"`
	SignatureApproved       bool   `dynamodbav:"signature_approved"`
	SignatureSigned         bool   `dynamodbav:"signature_signed"`
}

// MonthlyStats holds the count of signatures per month
type MonthlyStats struct {
	Month string
	ICLA  int
	ECLA  int
	CCLA  int
}

func main() {
	// Set up AWS session
	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: profileName,
		Config: aws.Config{
			Region: aws.String(regionDefault),
		},
	})
	if err != nil {
		log.Fatalf("Error creating AWS session: %v", err)
	}

	svc := dynamodb.New(sess)

	fmt.Println("Scanning signatures table for ICLA, ECLA, and CCLA statistics...")

	// Monthly counters map[YYYY-MM]Stats
	monthlyStats := make(map[string]*MonthlyStats)

	// Scan parameters
	// Full attributes scan
	// params := &dynamodb.ScanInput{TableName: aws.String(tableName)}
	// Scan only needed parameters
	params := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
		// Only fetch the fields we actually need
		ProjectionExpression: aws.String(
			"#sid, #dc, #adc, #st, #ssa, #sa, #ss",
		),
		ExpressionAttributeNames: map[string]*string{
			"#sid": aws.String("signature_id"),
			"#dc":  aws.String("date_created"),
			"#adc": aws.String("approx_date_created"),
			"#st":  aws.String("signature_type"),
			"#ssa": aws.String("sigtype_signed_approved_id"),
			"#sa":  aws.String("signature_approved"),
			"#ss":  aws.String("signature_signed"),
		},
	}

	// Get current time for validation
	now := time.Now()
	currentMonth := now.Format("2006-01")

	totalProcessed := 0
	totalICLA := 0
	totalECLA := 0
	totalCCLA := 0
	skippedInvalidDates := 0
	skippedFutureDates := 0

	// Scan the table
	err = svc.ScanPages(params, func(page *dynamodb.ScanOutput, lastPage bool) bool {
		for _, item := range page.Items {
			var sig SignatureRecord
			err := dynamodbattribute.UnmarshalMap(item, &sig)
			if err != nil {
				log.Printf("Error unmarshalling record: %v", err)
				continue
			}

			totalProcessed++
			if totalProcessed%1000 == 0 {
				fmt.Printf("Processed %d records...\n", totalProcessed)
			}

			// Only process signatures that are signed and approved
			if !sig.SignatureSigned || !sig.SignatureApproved {
				continue
			}

			// Get the creation date (prefer date_created, fallback to approx_date_created)
			creationDate := sig.DateCreated
			if creationDate == "" {
				creationDate = sig.ApproxDateCreated
			}
			if creationDate == "" {
				continue
			}

			// Parse creation date to extract month
			month := extractMonth(creationDate)
			if month == "" {
				skippedInvalidDates++
				continue
			}

			// Check if month is in the future
			// Month and currentMonth are formatted as YYYY-MM, so string comparison is safe
			if month > currentMonth {
				skippedFutureDates++
				continue
			}

			// Determine signature type based on multiple factors
			var isICLA, isECLA, isCCLA bool

			// Primary method: check sigtype_signed_approved_id
			if sig.SigtypeSignedApprovedID != "" {
				if strings.HasPrefix(sig.SigtypeSignedApprovedID, "icla#") {
					isICLA = true
					totalICLA++
				} else if strings.HasPrefix(sig.SigtypeSignedApprovedID, "ecla#") {
					isECLA = true
					totalECLA++
				} else if strings.HasPrefix(sig.SigtypeSignedApprovedID, "ccla#") {
					isCCLA = true
					totalCCLA++
				} else {
					// Skip unknown types
					continue
				}
			} else if sig.SignatureType != "" {
				// Fallback method: check signature_type field
				switch sig.SignatureType {
				case "cla", "icla":
					// For legacy CLA records without sigtype_signed_approved_id, treat as ICLA
					isICLA = true
					totalICLA++
				case "ccla":
					isCCLA = true
					totalCCLA++
				case "ecla":
					isECLA = true
					totalECLA++
				default:
					continue
				}
			} else {
				// Skip records without type information
				continue
			}

			// Initialize month stats if not exists
			if monthlyStats[month] == nil {
				monthlyStats[month] = &MonthlyStats{Month: month}
			}

			// Increment appropriate counter
			if isICLA {
				monthlyStats[month].ICLA++
			} else if isECLA {
				monthlyStats[month].ECLA++
			} else if isCCLA {
				monthlyStats[month].CCLA++
			}
		}
		return true // Continue scanning
	})

	if err != nil {
		log.Fatalf("Error scanning table: %v", err)
	}

	fmt.Printf("\nProcessing complete!\n")
	fmt.Printf("Total records processed: %d\n", totalProcessed)
	fmt.Printf("Total ICLA signatures: %d\n", totalICLA)
	fmt.Printf("Total ECLA signatures: %d\n", totalECLA)
	fmt.Printf("Total CCLA signatures: %d\n", totalCCLA)
	fmt.Printf("Skipped invalid dates: %d\n", skippedInvalidDates)
	fmt.Printf("Skipped future dates: %d\n", skippedFutureDates)

	// Convert map to slice and sort by month
	var monthlyData []MonthlyStats
	for _, stats := range monthlyStats {
		monthlyData = append(monthlyData, *stats)
	}

	sort.Slice(monthlyData, func(i, j int) bool {
		return monthlyData[i].Month < monthlyData[j].Month
	})

	// Create CSV output
	outputFile := "signature_monthly_report.csv"
	file, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("Error creating output file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)

	// Set semicolon as separator
	writer.Comma = ';'

	// Write header
	if err := writer.Write([]string{"month", "ICLAs", "ECLAs", "CCLAs"}); err != nil {
		log.Fatalf("Error writing CSV header: %v", err)
	}

	// Write data
	for _, stats := range monthlyData {
		record := []string{
			stats.Month,
			strconv.Itoa(stats.ICLA),
			strconv.Itoa(stats.ECLA),
			strconv.Itoa(stats.CCLA),
		}
		if err := writer.Write(record); err != nil {
			log.Fatalf("Error writing CSV record for month %s: %v", stats.Month, err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		log.Fatalf("Error flushing CSV writer: %v", err)
	}

	fmt.Printf("Report generated: %s\n", outputFile)
	fmt.Printf("Total months with activity: %d\n", len(monthlyData))
}

// extractMonth extracts YYYY-MM from date_created field with proper validation
func extractMonth(dateStr string) string {
	if dateStr == "" {
		return ""
	}

	// Handle different date formats
	// 2021-08-09T15:21:56.492368+0000
	// 2024-07-30T12:11:34Z

	var t time.Time
	var err error

	// Try parsing different formats
	formats := []string{
		"2006-01-02T15:04:05.999999+0000",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.999999Z",
		"2006-01-02T15:04:05+0000",
		"2006-01-02T15:04:05.999999-0700",
		"2006-01-02T15:04:05-0700",
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		t, err = time.Parse(format, dateStr)
		if err == nil {
			break
		}
	}

	thisYear := time.Now().Year()
	if err != nil {
		// Try to extract just the date part
		parts := strings.Split(dateStr, "T")
		if len(parts) > 0 {
			datePart := parts[0]
			if len(datePart) >= 7 { // YYYY-MM format at minimum
				// Try different date part lengths
				for _, length := range []int{10, 7} { // YYYY-MM-DD or YYYY-MM
					if len(datePart) >= length {
						testDateStr := datePart[:length]
						var testFormat string
						if length == 10 {
							testFormat = "2006-01-02"
						} else {
							testFormat = "2006-01"
						}

						if testTime, testErr := time.Parse(testFormat, testDateStr); testErr == nil {
							// Validate year and month ranges
							year := testTime.Year()
							month := int(testTime.Month())

							if year >= 2000 && year <= thisYear &&
								month >= 1 && month <= 12 {
								return testTime.Format("2006-01")
							}
						}
					}
				}
			}
		}
		return ""
	}

	// Validate the parsed time
	year := t.Year()
	month := int(t.Month())

	// Check for reasonable year and month ranges
	if year < 2000 || year > thisYear || month < 1 || month > 12 {
		return ""
	}

	result := t.Format("2006-01")

	// Additional validation: don't return invalid months like 2025-26
	if testTime, testErr := time.Parse("2006-01", result); testErr == nil {
		// Ensure the month is valid
		if testTime.Month() >= 1 && testTime.Month() <= 12 {
			return result
		}
	}

	return ""
}
