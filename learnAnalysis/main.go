package main

import (
	"github.com/Heph789/personalGoExperiments/learnAnalysis/sa"
	"golang.org/x/tools/go/analysis/singlechecker"

	"fmt"
)

func main() {
	singlechecker.Main(sa.Analyzer)
	fmt.Println("Finished.")
}
