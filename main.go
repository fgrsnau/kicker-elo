package main

func main() {
	Db.Initialize()
	go EloRunner()

	WebRun()
}
