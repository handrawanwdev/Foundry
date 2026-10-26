package main

import (
	"fmt"
	"os"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/spec"
	"github.com/handrawanwdev/foundry/internal/modules/work"
)

func main() {
	s, err := core.NewStore(os.Args[1])
	if err != nil {
		panic(err)
	}
	p, err := s.Load()
	if err != nil {
		panic(err)
	}
	fmt.Print(work.WorkflowPrompt(p, spec.Artifacts(p)))
}
