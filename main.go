package main

import (
	"encoding/csv"
	"fmt"
    "html/template"
	"log"
    "net/http"
    "slices"
	"math"
	"os"
    "mime/multipart"
    "io"
	"strings"
	"time"
)

type RA struct {
    name string
    conflicts map[Shift]bool
    primaries map[Shift]bool
    secondaries map[Shift]bool
    primary_score int
    secondary_score int
}

type Shift struct {
    date time.Time
    score int
}

var templates = template.Must(template.ParseFiles("templates/index.html"))

func main() {
    http.HandleFunc("/", homeHandler)
    http.HandleFunc("/calculate", calculateHandler)
    http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
    http.Handle("/examples/", http.StripPrefix("/examples/", http.FileServer(http.Dir("examples"))))
    log.Println("Server started on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))

}

func homeHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        templates.ExecuteTemplate(w, "index.html", nil)
        return
    }

    // Handle file uploads
    r.ParseMultipartForm(10 << 20) // 10MB max size

    holidayFile, _, err := r.FormFile("holidayFile")
    if err != nil {
        fmt.Fprintf(w, "Error uploading holidays file: %v", err)
        return
    }
    defer holidayFile.Close()

    raFile, _, err := r.FormFile("raFile")
    if err != nil {
        fmt.Fprintf(w, "Error uploading RAs file: %v", err)
        return
    }
    defer raFile.Close()

    datesFile, _, err := r.FormFile("datesFile")
    if err != nil {
        fmt.Fprintf(w, "Error uploading dates file: %v", err)
        return
    }
    defer datesFile.Close()

    saveUploadedFile(holidayFile, "holidays.txt")
    saveUploadedFile(raFile, "RAs.txt")
    saveUploadedFile(datesFile, "dates.txt")

    // Redirect to calculation handler
    http.Redirect(w, r, "/calculate", http.StatusSeeOther)
}

func saveUploadedFile(file multipart.File, filename string) error {
    outFile, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer outFile.Close()

    _, err = io.Copy(outFile, file)
    if err != nil {
        return err
    }

    return nil
}

func calculateHandler(w http.ResponseWriter, r *http.Request) {
    holiday_eves, err := handle_holidays("holidays.txt")
    if err != nil {
        print_error(err, w)
        return
    }
    ras, err := handle_RAs("RAs.txt", holiday_eves)
    if err != nil {
        print_error(err, w)
        return
    }
    start_date, end_date, err := handle_dates("dates.txt")
    if err != nil {
        print_error(err, w)
        return
    }

    schedule := fmt.Sprintf("Duration in days: %d\n", int(end_date.Sub(start_date).Hours() / 24))

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
        for shift := range ra.conflicts {
            if shift.date.Before(start_date) || shift.date.After(end_date) {
                schedule = fmt.Sprintf("RA %s's conflict not between start and end date", ra.name)
                templates.ExecuteTemplate(w, "index.html", schedule)
                return
            } 
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
            special_shifts[shift] = true
        }
        total_points += shift.score
    }

    schedule += fmt.Sprintf("Total points: %d\n", total_points)
    schedule += fmt.Sprintf("Number of RAs: %d\n", len(ras))

    target_points := int(math.Ceil(float64(total_points) / float64(len(ras))))

    schedule += fmt.Sprintf("Target per RA: %d\n", target_points)
    schedule += fmt.Sprintf("Unrounded Target: %f\n", float64(total_points) / float64(len(ras)))


    err = assign_primary_shifts(conflict_shifts, ras)
    if err != nil {
        print_error(err, w)
        return
    }
    err = assign_secondary_shifts(conflict_shifts, ras)
    if err != nil {
        print_error(err, w)
        return
    }

    err = assign_primary_shifts(special_shifts, ras)
    if err != nil {
        print_error(err, w)
        return
    }
    err = assign_secondary_shifts(special_shifts, ras)
    if err != nil {
        print_error(err, w)
        return
    }

    err = assign_primary_shifts(weekend_shifts, ras)
    if err != nil {
        print_error(err, w)
        return
    }
    err = assign_secondary_shifts(weekend_shifts, ras)
    if err != nil {
        print_error(err, w)
        return
    }

    err = assign_primary_shifts(sunday_shifts, ras)
    if err != nil {
        print_error(err, w)
        return
    }
    err = assign_secondary_shifts(sunday_shifts, ras)
    if err != nil {
        print_error(err, w)
        return
    }

    err = assign_primary_shifts(weekday_shifts, ras)
    if err != nil {
        print_error(err, w)
        return
    }
    err = assign_secondary_shifts(weekday_shifts, ras)
    if err != nil {
        print_error(err, w)
        return
    }

    schedule += dump_ra_info(ras)
    templates.ExecuteTemplate(w, "index.html", schedule)
    return
}

func print_error(err error, w http.ResponseWriter) {
    schedule := fmt.Sprintf("Error: %v", err)
    templates.ExecuteTemplate(w, "index.html", schedule)
}

func assign_primary_shifts(shift_set map[Shift]bool, ras []RA) (error) {
    for shift := range shift_set {
        err := assign_primary_shift(shift, ras)
        if err != nil {
            return fmt.Errorf("Error assigning primary shifts, check formatting or reduce number of conflicts.\nFailing Shift: %s", print_shift(shift))
        }
    }
    return nil
}

func assign_secondary_shifts(shift_set map[Shift]bool, ras []RA) (error) {
    for shift := range shift_set {
        err := assign_secondary_shift(shift, ras)
        if err != nil {
            return fmt.Errorf("Error assigning secondary shifts, check formatting or reduce number of conflicts.\nFailing Shift: %s", print_shift(shift))
        }
    }
    return nil
}

func remove_at_index(s []int, index int) []int {
    s[index] = s[len(s)-1]
    return s[:len(s)-1]
}

func index_of_lowest_ra_primary_score(ras []RA, indices []int) int {
    min := ras[indices[0]].primary_score
    min_index := 0

    for i, index := range indices {
        if i != 0 {
            if ras[index].primary_score < min {
                min = ras[index].primary_score
                min_index = i
            }
        }
    }
    return min_index
}

func assign_primary_shift(shift Shift, ras []RA) (error) {
    slices.SortFunc(ras, func(a, b RA) int {
        return a.primary_score - b.primary_score
    })

    for i := range ras {
        if !ras[i].conflicts[shift] {
            ras[i].primaries[shift] = true
            ras[i].primary_score += shift.score
            return nil
        }
    }
    return fmt.Errorf("Primary shift cannot be assigned to any RA")
}

func assign_secondary_shift(shift Shift, ras []RA) (error) {
    slices.SortFunc(ras, func(a, b RA) int {
        return a.secondary_score - b.secondary_score
    })

    for i := range ras {
        if !ras[i].primaries[shift] && !ras[i].conflicts[shift] {
            ras[i].secondaries[shift] = true
            ras[i].secondary_score += shift.score
            return nil
        }
    }
    return fmt.Errorf("Primary shift cannot be assigned to any RA")
}

func handle_RAs(filename string, holiday_eves map[time.Time]bool) (ras []RA, err error) {
    // Open the file
    file, err := os.Open(filename)
    if err != nil {
        return ras, err
    }
    defer file.Close()

    reader := csv.NewReader(file)
    reader.FieldsPerRecord = -1 // variable fields per line
    reader.TrimLeadingSpace = true // leading whitespace is ignored

    data, err := reader.ReadAll()
    if err != nil {
        return ras, err
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

        next_ra.conflicts = make(map[Shift]bool)
        for _, col := range row[1:] {
            t, err := date_from_string(col)
            if err != nil {
                return ras, err
            }
            next_ra.conflicts[shift_from_date(t, holiday_eves)] = true
        }
        // Add complete RA object to return list
        ras = append(ras, next_ra)
    }

    return ras, nil
}

func handle_dates(filename string) (start_date, end_date time.Time, err error) {
    // Open the file
    file, err := os.Open(filename)
    if err != nil {
        return time.Time{}, time.Time{}, err
    }
    defer file.Close()

    reader := csv.NewReader(file)
    reader.FieldsPerRecord = 1 // 1 date per line

    date, err := reader.Read()
    if err != nil {
        return time.Time{}, time.Time{}, err
    }

    start_date, err = date_from_string(date[0]);
    if err != nil {
        return time.Time{}, time.Time{}, err
    }

    date, err = reader.Read()
    if err != nil {
        return start_date, time.Time{}, err
    }

    end_date, err = date_from_string(date[0]);
    if err != nil {
        return start_date, time.Time{}, err
    }

    return start_date, end_date, nil
}

func handle_holidays(filename string) (holiday_eves map[time.Time]bool, err error) {
    // Open the file
    file, err := os.Open(filename)
    if err != nil {
        return make(map[time.Time]bool), err
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
        t, err := date_from_string(row[0])
        if err != nil {
            return make(map[time.Time]bool), err
        }
        t.AddDate(0, 0, -1)

        // Add complete RA object to return list
        holiday_eves[t] = true
    }

    return holiday_eves, nil
}

func date_from_string(date string) (time.Time, error) {
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
            return time.Time{}, fmt.Errorf("date not well-formatted! should be MM/DD or MM/DD/YYYY")
    }

    date = strings.Join(parts, "/")
    t, err := time.Parse(dateFormat, date)

    if err != nil {
        return t, err
    }

    if t.Year() == 0 {
        t = t.AddDate(time.Now().Year(), 0, 0)
    }

    return t, nil
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

func print_shift(shift Shift) (string) {
    return fmt.Sprintf("%s, %d points\n", shift.date.Format("01/02/2006"), shift.score)
}

func dump_ra_info (ras []RA) (string) {
    ret := fmt.Sprintln()
    for _, ra := range ras {
        ret += fmt.Sprintf("RA %s\n", ra.name)
        ret += fmt.Sprintf("  primary points: %d\n", ra.primary_score)
        for shift := range ra.primaries {
            ret += fmt.Sprint(shift.date.Format("01/02/2006"), ", ")
        }
        ret += fmt.Sprintf("\n  secondary points: %d\n", ra.secondary_score)
        for shift := range ra.secondaries {
            ret += fmt.Sprint(shift.date.Format("01/02/2006"), ", ")
        }
        ret += fmt.Sprintln()
    }
    return ret
}
