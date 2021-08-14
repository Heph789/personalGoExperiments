package main

import (
	// "github.com/Heph789/personalGoExperiments/learnAnalysis/sa"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(Analyzer)
}
