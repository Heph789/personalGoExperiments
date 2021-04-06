package main

import (
	"fmt"
	"os"
	"strconv"
)

func checkTemperature() {
	if len(os.Args) > 1 {
		temperatureToday, _ = strconv.Atoi(os.Args[1])
	}
	if temperature == temperatureToday {
		fmt.Print("Temperature is ideal.")
	} else {
		fmt.Print("Temperature is not ideal.")
	}
	fmt.Println()
}
