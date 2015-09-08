package main

import (
	"fmt"
	"time"
)

type ball struct {
	Hits int
}

func main() {
	table := make(chan *ball)
	go player("ping", table)
	go player("pong", table)

	ball := new(ball)
	table <- ball
	time.Sleep(1000 * time.Millisecond)
	<-table
}

func player(name string, table chan *ball) {
	for {
		ball := <-table
		ball.Hits++
		fmt.Printf("%s %d\n", name, ball.Hits)
		time.Sleep(100 * time.Millisecond)
		table <- ball
	}
}
