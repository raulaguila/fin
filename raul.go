package main

import "fmt"

func main() {
	update := 1
	read := 2
	create := 4

	readCreate := read | create | update

	fmt.Println(readCreate)
}
