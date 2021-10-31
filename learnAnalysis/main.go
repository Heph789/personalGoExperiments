package main

import (
	"fmt"

	"github.com/Heph789/personalGoExperiments/learnAnalysis/sa"

	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	fmt.Println("-----------------\n-----------------\n-----------------\n-----------------\n-----------------")
	singlechecker.Main(sa.Analyzer)
}
