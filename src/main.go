package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
)

// Tambahan fungsi untuk template
func addOne(i int) int {
	return i + 1
}

func mod(a, b int) bool {
	return a%b == 0
}

// Utility functions
func add(a, b int) int {
	return a + b
}

func sub(a, b int) int {
	return a - b
}

type libraryProfile struct {
	Name             string `json:"namaPerpus"`
	LibraryType      string `json:"jenisPerpus"`
	Country          string `json:"negara"`
	Province         string `json:"provinsi"`
	IdProvince       string `json:"idProvinsi"`
	RegistrationCode string `json:"kodeRegis"`
	IpAddress        string `json:"ip"`
}
type Library struct {
	DisplayIndex     int
	Name             string
	Type             string
	Province         string
	RegistrationCode string
	Year             string
}

type PageData struct {
	SearchQuery string
	Libraries   []Library
	CurrentPage int
	TotalPages  int
	PageSize    int
}

// Struktur respons JSON
type Response struct {
	Status  string `json:"status"`
	Message any    `json:"message"`
}

func main() {
	// Rute untuk file statis (CSS)
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	// Rute utama
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/registrasi", handleRegistrations)
	http.HandleFunc("/download", downloadCSV)
	fmt.Println("Starting server at port 8080")
	http.ListenAndServe(":8080", nil)
}

func connect() (*sql.DB, error) {
	db, err := sql.Open("mysql", "root:basdat21@tcp(127.0.0.1:3306)/registrasi")
	if err != nil {
		return nil, err
	}
	return db, nil
}

func paginate(libraries []Library, page, pageSize int) ([]Library, int) {
	totalItems := len(libraries)
	totalPages := int(math.Ceil(float64(totalItems) / float64(pageSize)))

	start := (page - 1) * pageSize
	if start > totalItems {
		start = totalItems
	}
	end := start + pageSize
	if end > totalItems {
		end = totalItems
	}

	return libraries[start:end], totalPages
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	db, err := connect()
	defer db.Close()
	readQuery := "SELECT namaPerpustakaan, jenisPerpustakaan, provinsi, kodeRegis, createdAt from registrasi_perpustakaan"
	rows, err := db.Query(readQuery)
	if err != nil {
		http.Error(w, "Gagal mengambil data", http.StatusInternalServerError)
		return
	}
	var libraries []Library
	searchQuery := r.URL.Query().Get("search")
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("pageSize")
	// Default pagination values
	page := 1
	pageSize := 20

	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	filteredLibraries := libraries
	for rows.Next() {
		var lib Library
		if err := rows.Scan(&lib.Name, &lib.Type, &lib.Province, &lib.RegistrationCode, &lib.Year); err != nil {
			http.Error(w, "Gagal membaca data", http.StatusInternalServerError)
			return
		}
		lib.Year = lib.Year[:4]
		if lib.Name != "" {
			libraries = append(libraries, lib)
		}
	}
	funcMap := template.FuncMap{
		"addOne": addOne,
		"mod":    mod,
		"sub":    sub,
		"add":    add,
	}
	tmpl := template.Must(template.New("index.html").Funcs(funcMap).ParseFiles("./templates/index.html"))
	if searchQuery != "" {
		filteredLibraries = []Library{}
		for _, lib := range libraries {
			if strings.Contains(strings.ToLower(lib.Name), strings.ToLower(searchQuery)) ||
				strings.Contains(strings.ToLower(lib.Type), strings.ToLower(searchQuery)) ||
				strings.Contains(strings.ToLower(lib.Province), strings.ToLower(searchQuery)) ||
				strings.Contains(lib.RegistrationCode, searchQuery) ||
				strings.Contains(lib.Year, searchQuery) {
				filteredLibraries = append(filteredLibraries, lib)
			}
		}
		// Paginate filtered libraries
		paginatedLibraries, totalPages := paginate(filteredLibraries, page, pageSize)
		// Populate the DisplayIndex for each library:
		for i := range paginatedLibraries {
			paginatedLibraries[i].DisplayIndex = ((page - 1) * pageSize) + (i + 1)
		}
		data := PageData{
			SearchQuery: searchQuery,
			Libraries:   paginatedLibraries,
			CurrentPage: page,
			TotalPages:  totalPages,
			PageSize:    pageSize,
		}
		if err := tmpl.Execute(w, data); err != nil {
			fmt.Println(err)
			http.Error(w, "Gagal merender template", http.StatusInternalServerError)
		}
		return
	}

	// Paginate filtered libraries
	paginatedLibraries, totalPages := paginate(libraries, page, pageSize)
	// Populate the DisplayIndex for each library:
	for i := range paginatedLibraries {
		paginatedLibraries[i].DisplayIndex = ((page - 1) * pageSize) + (i + 1)
	}
	data := PageData{
		SearchQuery: searchQuery,
		Libraries:   paginatedLibraries,
		CurrentPage: page,
		TotalPages:  totalPages,
		PageSize:    pageSize,
	}
	if err := tmpl.Execute(w, data); err != nil {
		fmt.Println(err)
		http.Error(w, "Gagal merender template", http.StatusInternalServerError)
	}
}

func writeRegistrations(libraryProfile libraryProfile) error {
	db, err := connect()
	if err != nil {
		fmt.Println("Error connecting to database:", err.Error())
		return err
	}
	defer db.Close()
	var count int
	checkQuery := "SELECT COUNT(*) FROM registrasi_perpustakaan WHERE kodeRegis = ?"
	err = db.QueryRow(checkQuery, libraryProfile.RegistrationCode).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("activationCode '%s' already exists", libraryProfile.RegistrationCode)
	}
	query := "INSERT INTO registrasi_perpustakaan (kodeRegis, namaPerpustakaan, jenisPerpustakaan, negara, provinsi, idProvinsi,ip) VALUES (?, ?, ?, ?, ?, ?, ?)"
	_, err = db.Exec(query, libraryProfile.RegistrationCode, libraryProfile.Name, libraryProfile.LibraryType, libraryProfile.Country, libraryProfile.Province, libraryProfile.IdProvince, libraryProfile.IpAddress)
	return err
}

func handleRegistrations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "POST" {
		var name = r.FormValue("namaPerpus")
		var libraryType = r.FormValue("jenisPerpus")
		var country = r.FormValue("negara")
		var idProvince = r.FormValue("idProvinsi")
		var province = r.FormValue("provinsi")
		var registrationCode = r.FormValue("kodeRegis")
		var ipAddress = r.FormValue("ip")
		// Validation for empty fields
		log.Println(r.Form)
		if name == "" || libraryType == "" || country == "" || province == "" || idProvince == "" || registrationCode == "" || ipAddress == "" {
			// Return error response if any required field is empty
			http.Error(w, `{"status": "error", "message": "All fields are required"}`, http.StatusBadRequest)
			log.Println(r.Body)
			log.Println(r.Form)
			return
		}

		registration := libraryProfile{
			Name:             name,
			LibraryType:      libraryType,
			Country:          country,
			Province:         province,
			IdProvince:       idProvince,
			RegistrationCode: registrationCode,
			IpAddress:        ipAddress,
		}
		if err := writeRegistrations(registration); err != nil {
			log.Println("Insert error:", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		// Membuat respons JSON
		response := Response{
			Status:  "success",
			Message: "registrasi berhasil",
		}
		// Kirim respons 200 OK
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	} else if r.Method == "GET" {
		kodeRegis := r.URL.Query().Get("noReg")
		if kodeRegis == "" {
			http.Error(w, "Missing query parameter", http.StatusBadRequest)
			return
		}
		lib, err := getLibrary(kodeRegis)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		// jsonData, err := json.Marshal(lib)
		// if err != nil {
		// 	http.Error(w, err.Error(), http.StatusInternalServerError)
		// 	return
		// }

		response := Response{
			Status:  "success",
			Message: lib,
		}
		// Kirim respons 200 OK
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}
func getLibrary(kodeRegis string) (libraryProfile, error) {
	var result = libraryProfile{}
	db, err := connect()
	if err != nil {
		fmt.Println("Error connecting to database:", err.Error())
		return result, err
	}
	defer db.Close()

	err = db.QueryRow("SELECT namaPerpustakaan, kodeRegis, negara, ip, jenisPerpustakaan, provinsi, idProvinsi FROM registrasi_perpustakaan WHERE kodeRegis = ?", kodeRegis).Scan(&result.Name, &result.RegistrationCode, &result.Country, &result.IpAddress, &result.LibraryType, &result.Province, &result.IdProvince)
	if err != nil {
		fmt.Println(err.Error())
		return result, err
	}
	return result, err

}

func downloadCSV(w http.ResponseWriter, r *http.Request) {
	// Connect to the database
	db, err := connect()
	if err != nil {
		http.Error(w, "Error connecting to database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Query data from the database
	rows, err := db.Query("SELECT id, kodeRegis, namaPerpustakaan, jenisPerpustakaan, negara, provinsi, idProvinsi, ip, createdAt FROM registrasi_perpustakaan")
	if err != nil {
		http.Error(w, "Error querying database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Set HTTP headers to force a download
	w.Header().Set("Content-Disposition", "attachment; filename=data.csv")
	w.Header().Set("Content-Type", "text/csv")

	// Create a CSV writer that writes directly to the HTTP response writer
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write the CSV header
	err = writer.Write([]string{"id", "kodeRegis", "namaPerpustakaan", "jenisPerpustakaan", "negara", "provinsi", "idProvinsi", "ip", "createdAt"})
	if err != nil {
		http.Error(w, "Error writing CSV header: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Write rows to the CSV
	for rows.Next() {
		var id, kodeRegis, namaPerpustakaan, jenisPerpustakaan, negara, provinsi, idProvinsi, ip, createdAt string
		if err := rows.Scan(&id, &kodeRegis, &namaPerpustakaan, &jenisPerpustakaan, &negara, &provinsi, &idProvinsi, &ip, &createdAt); err != nil {
			http.Error(w, "Error scanning row: "+err.Error(), http.StatusInternalServerError)
			return
		}
		err = writer.Write([]string{id, kodeRegis, namaPerpustakaan, jenisPerpustakaan, negara, provinsi, idProvinsi, ip, createdAt})
		if err != nil {
			http.Error(w, "Error writing row to CSV: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Check for errors during iteration
	if err := rows.Err(); err != nil {
		http.Error(w, "Error iterating rows: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
