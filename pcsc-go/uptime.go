package main

import "fmt"

func formatUptime(seconds uint64) string {
	total := seconds
	days := total / 86400
	total %= 86400
	hours := total / 3600
	total %= 3600
	minutes := total / 60
	secs := total % 60
	return fmt.Sprintf("%d days %d hours %d minutes %d seconds", days, hours, minutes, secs)
}
