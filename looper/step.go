// Copyright (c) 2022, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package looper

import (
	"fmt"
	"github.com/emer/emergent/etime"
	"strconv"
)

var printControlFlow = true

type Stepper struct {
	StopFlag       bool         `desc:"If true, stop model ASAP."`
	StopNext       bool         `desc:"If true, stop model after next stop level."`
	StopLevel      etime.Times  `desc:"Time level to stop at the end of."`
	StepIterations int          `desc:"How many steps to do."`
	Loops          *LoopManager `desc:"The information about loops."`
	Mode           etime.Modes  `desc:"The current evaluation mode."`

	// For internal use
	lastStoppedLevel int `desc:"The level at which a stop interrupted flow."`
	internalStop     bool
}

func (stepper *Stepper) Init(loopman *LoopManager) {
	stepper.Loops = loopman
	stepper.StopLevel = etime.Run
	stepper.Mode = etime.Train
	stepper.lastStoppedLevel = -1
}

func (stepper *Stepper) Run() {
	// Reset internal variables
	stepper.internalStop = false

	// 0 Means the top level loop, probably Run
	stepper.runLevel(0)
}

// runLevel implements nested for loops recursively. It is set up so that it can be stopped and resumed at any point.
func (stepper *Stepper) runLevel(currentLevel int) bool {
	//stepper.StopFlag = false // TODO Will this not work right?
	st := stepper.Loops.Stacks[stepper.Mode]
	if currentLevel >= len(st.Order) {
		return true // Stack overflow, expected at bottom of stack.
	}
	time := st.Order[currentLevel]
	loop := st.Loops[time]
	ctr := loop.Counter

	for ctr.Cur < ctr.Max || ctr.Max < 0 { // Loop forever for negative maxes
		stopAtLevel := st.Order[currentLevel] == stepper.StopLevel // Based on conversion of etime.Times to int
		if stepper.StopFlag && stopAtLevel {
			stepper.internalStop = true
			stepper.lastStoppedLevel = currentLevel
		}
		if stepper.internalStop {
			// This should occur before ctr incrementing and before functions.
			stepper.StopFlag = false
			return false // Don't continue above, e.g. Stop functions
		}
		if stepper.StopNext && st.Order[currentLevel] == stepper.StopLevel {
			stepper.StepIterations -= 1
			if stepper.StepIterations <= 0 {
				stepper.StopNext = false
				stepper.StopFlag = true
				stepper.lastStoppedLevel = -1
			}
		}

		if currentLevel >= stepper.lastStoppedLevel {
			// Loop flow was interrupted, and we should not start again.
			stepper.lastStoppedLevel = -1
			if printControlFlow && time > etime.Trial {
				fmt.Println(time.String() + ":Start:" + strconv.Itoa(ctr.Cur))
			}
			for _, fun := range loop.OnStart {
				fun.Func()
			}
		}

		// Recursion!
		stepper.phaseLogic(loop)
		runComplete := stepper.runLevel(currentLevel + 1)

		if runComplete {
			for _, fun := range loop.Main {
				fun.Func()
			}
			if printControlFlow && time > etime.Trial {
				fmt.Println(time.String() + ":End:  " + strconv.Itoa(ctr.Cur))
			}
			for _, fun := range loop.OnEnd {
				fun.Func()
			}
			for name, fun := range loop.IsDone {
				if fun() {
					_ = name // For debugging
					ctr.Cur = 0
					goto exitLoop // Exit multiple for-loops without flag variable.
				}
			}
			ctr.Cur = ctr.Cur + 1 // Increment
		}
	}

exitLoop:
	// Only get to this point if this loop is done.
	if !stepper.internalStop {
		ctr.Cur = 0
	}
	return true
}

// phaseLogic a loop can be broken up into discrete segments, so in a certain window you may want distinct behavior
func (stepper *Stepper) phaseLogic(loop *LoopStructure) {
	ctr := loop.Counter
	amount := 0
	for _, phase := range loop.Phases {
		amount += phase.Duration
		if ctr.Cur == (amount - phase.Duration) { //if start of a phase
			for _, function := range phase.PhaseStart {
				function.Func()
			}
		}
		if ctr.Cur < amount { //In between on Start and on End, inclusive
			for _, function := range phase.OnMillisecondEnd {
				function.Func()
			}
		}
		if ctr.Cur == amount-1 { //if end of a phase
			for _, function := range phase.PhaseEnd {
				function.Func()
			}
		}
	}
}
