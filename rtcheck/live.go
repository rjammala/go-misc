package main

import (
	"log"
	"os"

	"golang.org/x/tools/go/ssa"
)

// livenessFor computes which values must be live at entry to each
// basic block in f in order to compute each value in vals. deps is
// indexed by basic block number in f.
//
// TODO: This doesn't account for control flow dependencies. For
// example if a value depends on a phi, this will add all of the
// values that go into that phi, but not the values necessary to
// determine the control flow into that phi.
func livenessFor(f *ssa.Function, vals []ssa.Instruction) (deps []map[ssa.Instruction]struct{}) {
	deps = make([]map[ssa.Instruction]struct{}, len(f.Blocks))

	// For each operand to val, keep the operand live in all
	// blocks between the operand's definition and here.
	var walk func(def ssa.Instruction, use *ssa.BasicBlock)
	walk = func(def ssa.Instruction, use *ssa.BasicBlock) {
		if _, ok := deps[use.Index][def]; ok {
			return
		}
		if def.Block() == use {
			// We've reached the defining block.
			return
		}

		if deps[use.Index] == nil {
			deps[use.Index] = make(map[ssa.Instruction]struct{})
		}
		deps[use.Index][def] = struct{}{}

		if len(use.Preds) == 0 {
			f.WriteTo(os.Stderr)
			log.Fatalf("failed to find definition of %v", def)
		}
		for _, pred := range use.Preds {
			walk(def, pred)
		}
	}

	visited := make(map[ssa.Instruction]struct{})
	var doVal func(val ssa.Instruction)
	doVal = func(val ssa.Instruction) {
		if _, ok := visited[val]; ok {
			return
		}
		visited[val] = struct{}{}

		if phi, ok := val.(*ssa.Phi); ok {
			// A phi is special, as usual. It only uses
			// each operand if it came from the
			// corresponding predecessor.
			if deps[phi.Block().Index] == nil {
				deps[phi.Block().Index] = make(map[ssa.Instruction]struct{})
			}
			for i, rand := range phi.Edges {
				rand, ok := rand.(ssa.Instruction)
				if !ok {
					continue
				}
				deps[phi.Block().Index][rand] = struct{}{}
				walk(rand, phi.Block().Preds[i])

				// Recursively depend on the inputs to
				// this operand.
				doVal(rand)
			}
		} else {
			// Regular instruction uses all of their operands.
			rands := val.Operands(nil)
			for _, rand := range rands {
				rand, ok := (*rand).(ssa.Instruction)
				if !ok {
					continue
				}
				walk(rand, val.Block())

				// Recursively depend on the inputs to
				// the operands.
				doVal(rand)
			}
		}
	}

	for _, val := range vals {
		doVal(val)
	}
	return deps
}