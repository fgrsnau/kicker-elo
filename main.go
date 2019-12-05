package main

func main() {
	Db.Initialize()
	go Db.StatsRunner()
	go EloRunner()
	WebRun()
}
