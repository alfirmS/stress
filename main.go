package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// UserStats menyimpan statistik khusus untuk setiap user
type UserStats struct {
	TotalQueries  int
	TotalTime     time.Duration
	LongestTime   time.Duration
	ShortestTime  time.Duration
	LongestQuery  string
	ShortestQuery string
}

// QueryStats menyimpan statistik global termasuk statistik untuk setiap user
type QueryStats struct {
	TotalQueries      int
	QueriesPerUser    map[int]UserStats
	AverageQueryTime  time.Duration
	MaxQueryTime      time.Duration
	MinQueryTime      time.Duration
	FailedQueryCount  int
	SuccessfulQueries int
	StartTime         time.Time
}

// runQuery mengumpulkan statistik khusus user untuk setiap query
func runQuery(db *sql.DB, query string, interval time.Duration, concurrency, iteration int, wg *sync.WaitGroup, results chan<- time.Duration, stats *QueryStats) {
	defer wg.Done()

	for i := 0; i < iteration; i++ {
		for user := 1; user <= concurrency; user++ {
			startTime := time.Now()

			_, err := db.Exec(query)
			if err != nil {
				log.Println("Error executing query:", err)
				stats.FailedQueryCount++
			} else {
				stats.SuccessfulQueries++
			}

			elapsedTime := time.Since(startTime)

			stats.TotalQueries++
			// Buat instance baru dari UserStats jika belum ada untuk user tertentu
			userStats, ok := stats.QueriesPerUser[user]
			if !ok {
				userStats = UserStats{}
			}
			// Update statistik waktu eksekusi untuk user tertentu
			userStats.TotalQueries++
			userStats.TotalTime += elapsedTime

			// Perbarui statistik waktu eksekusi terpanjang dan terpendek
			if elapsedTime > userStats.LongestTime {
				userStats.LongestTime = elapsedTime
				// Simpan query dengan eksekusi paling lama
				userStats.LongestQuery = query
			}
			if elapsedTime < userStats.ShortestTime || userStats.ShortestTime == 0 {
				userStats.ShortestTime = elapsedTime
				// Simpan query dengan eksekusi paling cepat
				userStats.ShortestQuery = query
			}

			// Simulasi beberapa waktu pemrosesan
			time.Sleep(10 * time.Millisecond)

			// Set userStats kembali ke peta
			stats.QueriesPerUser[user] = userStats
		}
		time.Sleep(interval)
	}

	elapsedTime := time.Since(stats.StartTime)
	results <- elapsedTime

	updateQueryTimeStats(stats, elapsedTime)
}

// updateQueryTimeStats memperbarui statistik waktu eksekusi global
func updateQueryTimeStats(stats *QueryStats, elapsedTime time.Duration) {
	if elapsedTime > stats.MaxQueryTime {
		stats.MaxQueryTime = elapsedTime
	}

	if elapsedTime < stats.MinQueryTime || stats.MinQueryTime == 0 {
		stats.MinQueryTime = elapsedTime
	}

	stats.AverageQueryTime = time.Duration(int64(stats.AverageQueryTime)*int64(stats.TotalQueries-1)/int64(stats.TotalQueries) + int64(elapsedTime)/int64(stats.TotalQueries))
}

// stressTest mensimulasikan uji stres pada database
func stressTest(host, username, password, database, query string, interval time.Duration, concurrency, iteration int, results chan<- time.Duration, stats *QueryStats) {
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/%s", username, password, host, database))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go runQuery(db, query, interval, concurrency, iteration, &wg, results, stats)
	}

	wg.Wait()
	close(results)
}

// printQueriesPerUser menampilkan query yang dieksekusi per user
func printQueriesPerUser(queriesPerUser map[int]UserStats) {
	fmt.Println("2. Queries per user:")
	fmt.Println("   User   |   Total Queries   |   Average Time   |   Fastest Time   |   Slowest Time")
	for user, userStats := range queriesPerUser {
		averageTime := userStats.TotalTime / time.Duration(userStats.TotalQueries)
		fmt.Printf("   %d     |   %d              |   %s           |   %s           |   %s\n", user, userStats.TotalQueries, averageTime, userStats.ShortestTime, userStats.LongestTime)
	}
}

// printLongestShortestQueries menampilkan query dengan execution time paling lama dan paling cepat
func printLongestShortestQueries(stats *QueryStats) {
	fmt.Printf("4. Longest query execution time: %s\n", stats.MaxQueryTime)
	fmt.Printf("5. Shortest query execution time: %s\n", stats.MinQueryTime)
}

// printResults menampilkan hasil dan statistik
func printResults(stats *QueryStats, startTime, endTime time.Time) {
	fmt.Println("Results:")
	fmt.Printf("1. Total queries executed: %d\n", stats.TotalQueries)
	printQueriesPerUser(stats.QueriesPerUser)
	fmt.Printf("3. Average query completion time: %s\n", stats.AverageQueryTime)
	fmt.Printf("4. Number of unsuccessful queries (in percentage): %.2f%%\n", calculatePercentage(stats.FailedQueryCount, stats.TotalQueries))
	fmt.Printf("5. Number of successful queries (in percentage): %.2f%%\n", calculatePercentage(stats.SuccessfulQueries, stats.TotalQueries))
	fmt.Printf("6. CLI start time: %s\n", startTime.Format(time.RFC3339))
	fmt.Printf("7. CLI end time: %s\n", endTime.Format(time.RFC3339))
}

// calculatePercentage menghitung persentase dari query yang berhasil atau tidak berhasil
func calculatePercentage(count, total int) float64 {
	if total == 0 {
		return 0.0
	}
	return float64(count) / float64(total) * 100
}

func main() {
	var host, username, password, database, customQuery string
	var testInterval time.Duration
	var concurrency, iteration int

	flag.StringVar(&host, "host", "localhost:3306", "Database host")
	flag.StringVar(&username, "user", "root", "Database username")
	flag.StringVar(&password, "password", "", "Database password")
	flag.StringVar(&database, "database", "your_database_name", "Database name")
	flag.StringVar(&customQuery, "query", "", "Custom database query")
	flag.DurationVar(&testInterval, "interval", 10*time.Minute, "Interval between queries")
	flag.IntVar(&concurrency, "concurrency", 10, "Number of concurrent users")
	flag.IntVar(&iteration, "iteration", 5, "Number of iterations")

	flag.Parse()

	if customQuery == "" {
		fmt.Println("Please provide a custom query using the -query flag.")
		os.Exit(1)
	}

	stats := &QueryStats{
		QueriesPerUser: make(map[int]UserStats),
		StartTime:      time.Now(),
	}

	results := make(chan time.Duration, concurrency*iteration)
	go stressTest(host, username, password, database, customQuery, testInterval, concurrency, iteration, results, stats)

	var resultDurations []time.Duration
	for r := range results {
		resultDurations = append(resultDurations, r)
	}

	endTime := time.Now()
	printResults(stats, stats.StartTime, endTime)
}

