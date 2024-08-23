package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"
    "math"
)

type RA struct {
    name string
    conflicts []time.Time
    primaries map[Shift]bool
    secondaries map[Shift]bool
    primary_score int
    secondary_score int
}

type Shift struct {
    date time.Time
    score int
}

func main() {
    ras := handle_RAs("RAs.txt")
    holiday_eves := handle_holidays("holidays.txt")
    start_date, end_date := handle_dates("dates.txt")

    fmt.Printf("Duration in days: %d\n", int(end_date.Sub(start_date).Hours() / 24))

    // Shifts divided by points so higher-value shifts are assigned first (prevent weird behavior)
    weekend_shifts := make(map[Shift]bool)
    sunday_shifts := make(map[Shift]bool)
    weekday_shifts := make(map[Shift]bool)

    // Special container for shifts given extra value (not currently implemented)
    special_shifts := make(map[Shift]bool)

    // Container for shifts that not everyone can perform (should be given priority)
    conflict_shifts := make(map[Shift]bool)

    /* Sort each shift into category, tracking total points assigned */
    total_points := 0

    for _, ra := range ras {
        for _, d := range ra.conflicts {
            if d.Before(start_date) || d.After(end_date) {
                log.Fatal(fmt.Sprintf("RA %s: Conflict not between start and end date",
                    ra.name))
            } 
            shift := shift_from_date(d, holiday_eves)

            // Don't double-count conflict shifts
            if conflict_shifts[shift] { continue }

            conflict_shifts[shift] = true
            total_points += shift.score
        }
    }

    for d := start_date; d.Before(end_date); d = d.Add(24 * time.Hour) {
        shift := shift_from_date(d, holiday_eves)

        // Don't double-count conflict shifts
        if conflict_shifts[shift] { continue }

        switch shift.score {
        case 3:
            weekend_shifts[shift] = true
        case 2:
            sunday_shifts[shift] = true
        case 1:
            weekday_shifts[shift] = true
        default:
            fmt.Print("Special shift detected: ")
            print_shift(shift)
            special_shifts[shift] = true
        }
        total_points += shift.score
    }

    fmt.Printf("Total points: %d\n", total_points)
    fmt.Printf("Number of RAs: %d\n", len(ras))

    target_points := int(math.Ceil(float64(total_points) / float64(len(ras))))

    fmt.Printf("Target per RA: %d\n", target_points)
    fmt.Printf("Unrounded Target: %f\n", float64(total_points) / float64(len(ras)))


    assign_primary_shifts(conflict_shifts, ras, target_points)
    assign_primary_shifts(special_shifts, ras, target_points)
    assign_primary_shifts(weekend_shifts, ras, target_points)
    assign_primary_shifts(sunday_shifts, ras, target_points)
    assign_primary_shifts(weekday_shifts, ras, target_points)

    assign_secondary_shifts(conflict_shifts, ras, target_points)
    assign_secondary_shifts(special_shifts, ras, target_points)
    assign_secondary_shifts(weekend_shifts, ras, target_points)
    assign_secondary_shifts(sunday_shifts, ras, target_points)
    assign_secondary_shifts(weekday_shifts, ras, target_points)

    dump_ra_info(ras)
}

func assign_primary_shifts(shift_set map[Shift]bool, ras []RA, target_points int) {
    for shift := range shift_set {
        // To prevent infinite loops or slowdowns, method for not re-testing bad fits
        valid_ras := make([]int, 0, len(ras))
        for i := range ras {
            valid_ras = append(valid_ras, i)
        }

        for true {
            random_index := rand.Intn(len(valid_ras))
            valid_index := valid_ras[random_index]

            if ras[valid_index].primary_score + shift.score > target_points {
                valid_ras = remove_at_index(valid_ras, random_index)
                if len(valid_ras) == 0 {
                    dump_ra_info(ras)
                    fmt.Println("Failing shift:")
                    print_shift(shift)
                    log.Fatal("Primary shifts not able to be assigned properly")
                }
            } else {
                // Add shift to ra's list of primaries
                ras[valid_index].primaries[shift] = true
                // Add point value of shift to ra's score
                ras[valid_index].primary_score += shift.score

                break
            }
        }
    }
}

func assign_secondary_shifts(shift_set map[Shift]bool, ras []RA, target_points int) {
    for shift := range shift_set {
        // To prevent infinite loops or slowdowns, method for not re-testing bad fits
        valid_ras := make([]int, 0, len(ras))
        for i := range ras {
            valid_ras = append(valid_ras, i)
        }

        for true {
            random_index := rand.Intn(len(valid_ras))
            valid_index := valid_ras[random_index]

            if ras[valid_index].secondary_score + shift.score > target_points ||
            ras[valid_index].primaries[shift] {
                valid_ras = remove_at_index(valid_ras, random_index)
                if len(valid_ras) == 0 {
                    dump_ra_info(ras)
                    fmt.Println("Failing shift:")
                    print_shift(shift)
                    log.Fatal("Secondary shifts not able to be assigned properly")
                }
            } else {
                // Add shift to ra's list of secondaries
                ras[valid_index].secondaries[shift] = true
                // Add point value of shift to ra's score
                ras[valid_index].secondary_score += shift.score

                break
            }
        }
    }
}

func remove_at_index(s []int, index int) []int {
    s[index] = s[len(s)-1]
    return s[:len(s)-1]
}

func handle_RAs(filename string) (ras []RA) {
    // Open the file
    file, err := os.Open(filename)
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
        next_ra.primaries = make(map[Shift]bool)
        next_ra.secondaries = make(map[Shift]bool)

        for _, col := range row[1:] {
            next_ra.conflicts = append(next_ra.conflicts, time_from_date(col))
        }
        // Add complete RA object to return list
        ras = append(ras, next_ra)
    }

    return ras
}

func handle_dates(filename string) (start_date, end_date time.Time) {
    // Open the file
    file, err := os.Open(filename)
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

    return start_date, end_date
}

func handle_holidays(filename string) (holiday_eves map[time.Time]bool) {
    // Open the file
    file, err := os.Open(filename)
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()

    reader := csv.NewReader(file)
    reader.FieldsPerRecord = 1 // 1 date per line

    data, err := reader.ReadAll()
    if err != nil {
        log.Fatal(err)
    }

    holiday_eves = make(map[time.Time]bool)

    for _, row := range data {
        t := time_from_date(row[0])
        t.AddDate(0, 0, -1)

        // Add complete RA object to return list
        holiday_eves[t] = true
    }

    return holiday_eves
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

func shift_from_date(d time.Time, holiday_eves map[time.Time]bool) Shift {
    if d.Weekday() == time.Friday || d.Weekday() == time.Saturday {
        return Shift{date: d, score: 3}
    } else if holiday_eves[d] || d.Weekday() == time.Sunday {
        return Shift{date: d, score: 2}
    } else {
        return Shift{date: d, score: 1}
    }
}

func print_shift(shift Shift) {
    fmt.Print(shift.date.Format("01/02/2006"))
    fmt.Printf(", %d points\n", shift.score)
}

func dump_ra_info (ras []RA) {
    for _, ra := range ras {
        fmt.Printf("RA %s\n", ra.name)
        fmt.Printf("  primary points: %d\n", ra.primary_score)
        for shift := range ra.primaries {
            fmt.Print(shift.date.Format("01/02/2006"), ", ")
        }
        fmt.Printf("\n  secondary points: %d\n", ra.secondary_score)
        for shift := range ra.secondaries {
            fmt.Print(shift.date.Format("01/02/2006"), ", ")
        }
        fmt.Println()
    }
}
