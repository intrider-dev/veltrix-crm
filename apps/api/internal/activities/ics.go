package activities

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

type CalendarItem struct {
	ID             string
	Type           string
	Title          string
	Body           string
	Location       string
	Status         string
	Start          time.Time
	End            *time.Time
	Due            *time.Time
	RecurrenceRule string
	UpdatedAt      time.Time
}

type CalendarExport struct {
	ProductName string
	ProductID   string
	GeneratedAt time.Time
	Items       []CalendarItem
}

func WriteICS(destination io.Writer, export CalendarExport) error {
	writer := bufio.NewWriter(destination)
	lines := []string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"CALSCALE:GREGORIAN",
		"METHOD:PUBLISH",
		"PRODID:-//" + escapeICSText(export.ProductID) + "//CRM Calendar//EN",
		"X-WR-CALNAME:" + escapeICSText(export.ProductName),
	}
	for _, line := range lines {
		if err := writeICSLine(writer, line); err != nil {
			return err
		}
	}
	stamp := export.GeneratedAt.UTC()
	for _, item := range export.Items {
		if item.Type == "task" {
			if err := writeTodo(writer, item, stamp); err != nil {
				return err
			}
			continue
		}
		if err := writeEvent(writer, item, stamp); err != nil {
			return err
		}
	}
	if err := writeICSLine(writer, "END:VCALENDAR"); err != nil {
		return err
	}
	return writer.Flush()
}

func writeEvent(writer *bufio.Writer, item CalendarItem, stamp time.Time) error {
	end := item.Start
	if item.End != nil {
		end = item.End.UTC()
	}
	lines := []string{
		"BEGIN:VEVENT",
		"UID:" + escapeICSText(item.ID),
		"DTSTAMP:" + icsTime(stamp),
		"DTSTART:" + icsTime(item.Start),
	}
	if item.End != nil {
		lines = append(lines, "DTEND:"+icsTime(end))
	}
	lines = append(lines,
		"SUMMARY:"+escapeICSText(item.Title),
		"DESCRIPTION:"+escapeICSText(item.Body),
		"LOCATION:"+escapeICSText(item.Location),
		"LAST-MODIFIED:"+icsTime(item.UpdatedAt),
	)
	if item.RecurrenceRule != "" {
		lines = append(lines, "RRULE:"+item.RecurrenceRule)
	}
	lines = append(lines, "END:VEVENT")
	return writeICSLines(writer, lines)
}

func writeTodo(writer *bufio.Writer, item CalendarItem, stamp time.Time) error {
	due := item.Start
	if item.Due != nil {
		due = item.Due.UTC()
	}
	status := "NEEDS-ACTION"
	if item.Status == "completed" {
		status = "COMPLETED"
	} else if item.Status == "cancelled" {
		status = "CANCELLED"
	}
	lines := []string{
		"BEGIN:VTODO",
		"UID:" + escapeICSText(item.ID),
		"DTSTAMP:" + icsTime(stamp),
		"DUE:" + icsTime(due),
		"SUMMARY:" + escapeICSText(item.Title),
		"DESCRIPTION:" + escapeICSText(item.Body),
		"STATUS:" + status,
		"LAST-MODIFIED:" + icsTime(item.UpdatedAt),
	}
	if item.RecurrenceRule != "" {
		lines = append(lines, "RRULE:"+item.RecurrenceRule)
	}
	lines = append(lines, "END:VTODO")
	return writeICSLines(writer, lines)
}

func writeICSLines(writer *bufio.Writer, lines []string) error {
	for _, line := range lines {
		if err := writeICSLine(writer, line); err != nil {
			return err
		}
	}
	return nil
}

func escapeICSText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, ";", "\\;")
	return strings.ReplaceAll(value, ",", "\\,")
}

func writeICSLine(writer *bufio.Writer, line string) error {
	if strings.ContainsAny(line, "\r\n") {
		return fmt.Errorf("ICS line contains an unescaped line break")
	}
	first := true
	for len(line) > 0 {
		limit := 75
		prefix := ""
		if !first {
			limit = 74
			prefix = " "
		}
		cut := utf8Prefix(line, limit)
		if _, err := writer.WriteString(prefix + line[:cut] + "\r\n"); err != nil {
			return err
		}
		line = line[cut:]
		first = false
	}
	if first {
		_, err := writer.WriteString("\r\n")
		return err
	}
	return nil
}

func utf8Prefix(value string, byteLimit int) int {
	if len(value) <= byteLimit {
		return len(value)
	}
	cut := 0
	for cut < len(value) {
		_, width := utf8.DecodeRuneInString(value[cut:])
		if cut+width > byteLimit {
			break
		}
		cut += width
	}
	return cut
}

func icsTime(value time.Time) string {
	return value.UTC().Format("20060102T150405Z")
}
