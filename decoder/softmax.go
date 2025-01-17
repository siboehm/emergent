// Copyright (c) 2021, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package decoder

import (
	"math"
	"sort"

	"github.com/emer/emergent/emer"
	"github.com/emer/etable/etensor"
	"github.com/goki/mat32"
)

// SoftMax is a softmax decoder, which is the best choice for a 1-hot classification
// using the widely-used SoftMax function: https://en.wikipedia.org/wiki/Softmax_function
type SoftMax struct {
	Lrate    float32                     `def:"0.1" desc:"learning rate"`
	Layers   []emer.Layer                `desc:"layers to decode"`
	NCats    int                         `desc:"number of different categories to decode"`
	Units    []SoftMaxUnit               `desc:"unit values"`
	Sorted   []int                       `desc:"sorted list of indexes into Units, in descending order from strongest to weakest -- i.e., Sorted[0] has the most likely categorization, and its activity is Units[Sorted[0]].Act"`
	NInputs  int                         `desc:"number of inputs -- total sizes of layer inputs"`
	Inputs   []float32                   `desc:"input values, copied from layers"`
	Target   int                         `desc:"current target index of correct category"`
	ValsTsrs map[string]*etensor.Float32 `view:"-" desc:"for holding layer values"`
	Weights  etensor.Float32             `desc:"synaptic weights: outer loop is units, inner loop is inputs"`
}

// SoftMaxUnit has variables for softmax decoder unit
type SoftMaxUnit struct {
	Act float32 `desc:"final activation = e^Ge / sum e^Ge"`
	Net float32 `desc:"net input = sum x * w"`
	Exp float32 `desc:"exp(Net)"`
}

// InitLayer initializes detector with number of categories and layers
func (sm *SoftMax) InitLayer(ncats int, layers []emer.Layer) {
	sm.Layers = layers
	nin := 0
	for _, ly := range sm.Layers {
		nin += ly.Shape().Len()
	}
	sm.Init(ncats, nin)
}

// Init initializes detector with number of categories and number of inputs
func (sm *SoftMax) Init(ncats, ninputs int) {
	sm.NInputs = ninputs
	sm.Lrate = 0.1 // seems pretty good
	sm.NCats = ncats
	sm.Units = make([]SoftMaxUnit, ncats)
	sm.Sorted = make([]int, ncats)
	sm.Inputs = make([]float32, sm.NInputs)
	sm.Weights.SetShape([]int{sm.NCats, sm.NInputs}, nil, []string{"Cats", "Inputs"})
	for i := range sm.Weights.Values {
		sm.Weights.Values[i] = .1
	}
}

// Decode decodes the given variable name from layers (forward pass)
// See Sorted list of indexes for the decoding output -- i.e., Sorted[0]
// is the most likely -- that is returned here as a convenience.
func (sm *SoftMax) Decode(varNm string) int {
	sm.Input(varNm)
	sm.Forward()
	sm.Sort()
	return sm.Sorted[0]
}

// Train trains the decoder with given target correct answer (0..NCats-1)
func (sm *SoftMax) Train(targ int) {
	sm.Target = targ
	sm.Back()
}

// ValsTsr gets value tensor of given name, creating if not yet made
func (sm *SoftMax) ValsTsr(name string) *etensor.Float32 {
	if sm.ValsTsrs == nil {
		sm.ValsTsrs = make(map[string]*etensor.Float32)
	}
	tsr, ok := sm.ValsTsrs[name]
	if !ok {
		tsr = &etensor.Float32{}
		sm.ValsTsrs[name] = tsr
	}
	return tsr
}

// Input grabs the input from given variable in layers
func (sm *SoftMax) Input(varNm string) {
	off := 0
	for _, ly := range sm.Layers {
		tsr := sm.ValsTsr(ly.Name())
		ly.UnitValsTensor(tsr, varNm)
		for j, v := range tsr.Values {
			sm.Inputs[off+j] = v
		}
		off += ly.Shape().Len()
	}
}

// Forward compute the forward pass from input
func (sm *SoftMax) Forward() {
	max := float32(-math.MaxFloat32)
	for ui := range sm.Units {
		u := &sm.Units[ui]
		net := float32(0)
		off := ui * sm.NInputs
		for j, in := range sm.Inputs {
			net += sm.Weights.Values[off+j] * in
		}
		u.Net = net
		if net > max {
			max = net
		}
	}
	sum := float32(0)
	for ui := range sm.Units {
		u := &sm.Units[ui]
		u.Net -= max
		u.Exp = mat32.FastExp(u.Net)
		sum += u.Exp
	}
	for ui := range sm.Units {
		u := &sm.Units[ui]
		u.Act = u.Exp / sum
	}
}

// Sort updates Sorted indexes of the current Unit category activations sorted
// from highest to lowest.  i.e., the 0-index value has the strongest
// decoded output category, 1 the next-strongest, etc.
func (sm *SoftMax) Sort() {
	for i := range sm.Sorted {
		sm.Sorted[i] = i
	}
	sort.Slice(sm.Sorted, func(i, j int) bool {
		return sm.Units[sm.Sorted[i]].Act > sm.Units[sm.Sorted[j]].Act
	})
}

// Back compute the backward error propagation pass
func (sm *SoftMax) Back() {
	lr := sm.Lrate
	for ui := range sm.Units {
		u := &sm.Units[ui]
		var del float32
		if ui == sm.Target {
			del = lr * (1 - u.Act)
		} else {
			del = -lr * u.Act
		}
		off := ui * sm.NInputs
		for j, in := range sm.Inputs {
			sm.Weights.Values[off+j] += del * in
		}
	}
}
