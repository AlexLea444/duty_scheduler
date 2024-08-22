package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"time"
    "strings"
)

type RA struct {
    name string
    conflicts []time.Time
    primary_score int
    secondary_score int
}

func main() {
    ras := handle_RAs("RAs.txt")
    start_date, end_date := handle_dates("dates.txt")

    for _, ra := range ras {
        fmt.Printf("Name: %s, Conflicts: ", ra.name)
        for _, d := range ra.conflicts {
            fmt.Print(d.Format("2006-Jan-02"), ", ")
        }
        fmt.Println()
    }

    for d := start_date; d.Before(end_date); d = d.Add(24 * time.Hour) {
        if d.Weekday() == time.Friday || d.Weekday() == time.Saturday {
            fmt.Print(d.Format("2006-Jan-02"))
        }
    }
}

func handle_RAs(ra_filename string) (ras []RA) {
    // Open the file
    file, err := os.Open(ra_filename)
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()

    reader := csv.NewReader(file)
    reader.FieldsPerRecord = -1 // variable fields per line
    reader.TrimLeadingSpace = true // leading whitespace is ignored

    data, err := reader.ReadAll()
    if err != nil {
        log.Fatal(err)
    }

    for _, row := range data {
        // Generate new RA per row
        next_ra := RA{}
        next_ra.name = row[0]
        
        // Default initialization (will be useful later)
        next_ra.primary_score = 0
        next_ra.secondary_score = 0

        for _, col := range row[1:] {
            next_ra.conflicts = append(next_ra.conflicts, time_from_date(col))
        }
        // Add complete RA object to return list
        ras = append(ras, next_ra)
    }

    return ras
}

func handle_dates(dates_filename string) (start_date, end_date time.Time) {
    // Open the file
    file, err := os.Open(dates_filename)
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()

    reader := csv.NewReader(file)
    reader.FieldsPerRecord = 1 // 1 date per line

    date, err := reader.Read()
    if err != nil {
        log.Fatal(err)
    }

    start_date = time_from_date(date[0]);

    date, err = reader.Read()
    if err != nil {
        log.Fatal(err)
    }

    end_date = time_from_date(date[0]);

    fmt.Printf("Duration in days: %f\n", end_date.Sub(start_date).Hours() / 24)

    return start_date, end_date
}

func time_from_date(date string) time.Time {
    // Split the date string into parts
    parts := strings.Split(date, "/")

    // Ensure the month and day parts have leading zeros
    if len(parts[0]) == 1 {
        parts[0] = "0" + parts[0]
    }
    if len(parts[1]) == 1 {
        parts[1] = "0" + parts[1]
    }

    var dateFormat string

    switch len(parts) {
        case 2:
            dateFormat = "01/02"
        case 3:
            if len(parts[2]) == 2 {
                parts[2] = "20" + parts[2]
            }
            dateFormat = "01/02/2006"
        default:
            log.Fatal("date not well-formatted! should be MM/DD or MM/DD/YYYY")
    }

    date = strings.Join(parts, "/")
    t, err := time.Parse(dateFormat, date)

    if err != nil {
        log.Fatal(err)
    }

    if t.Year() == 0 {
        t = t.AddDate(time.Now().Year(), 0, 0)
    }

    return t
}
