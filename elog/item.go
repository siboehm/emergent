// Copyright (c) 2022, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package elog

import (
	"github.com/emer/emergent/etime"
	"github.com/emer/etable/etensor"
	"github.com/emer/etable/minmax"
)

const (
	// DTrue is deprecated -- just use true
	DTrue = true
	// DFalse is deprecated -- just use false
	DFalse = false
)

// WriteMap holds log writing functions for scope keys
type WriteMap map[etime.ScopeKey]WriteFunc

// Item describes one item to be logged -- has all the info
// for this item, across all scopes where it is relevant.
type Item struct {
	Name      string       `desc:"name of column -- must be unique for a table"`
	Type      etensor.Type `desc:"data type, using etensor types which are isomorphic with arrow.Type"`
	CellShape []int        `desc:"shape of a single cell in the column (i.e., without the row dimension) -- for scalars this is nil -- tensor column will add the outer row dimension to this shape"`
	DimNames  []string     `desc:"names of the dimensions within the CellShape -- 'Row' will be added to outer dimension"`
	Write     WriteMap     `desc:"holds Write functions for different scopes.  After processing, the scope key will be a single mode and time, from Scope(mode, time), but the initial specification can lists for each, or the All* option, if there is a Write function that works across scopes"`
	Plot      bool         `desc:"Whether or not to plot it"`
	Range     minmax.F64   `desc:"The minimum and maximum values, for plotting"`
	FixMin    bool         `desc:"Whether to fix the minimum in the display"`
	FixMax    bool         `desc:"Whether to fix the maximum in the display"`
	ErrCol    string       `desc:"Name of other item that has the error bar values for this item -- for plotting"`
	TensorIdx int          `desc:"index of tensor to plot -- defaults to 0 -- use -1 to plot all"`
	Color     string       `desc:"specific color for plot -- uses default ordering of colors if empty"`

	// following are updated in final Process step
	Modes map[string]bool `desc:"map of eval modes that this item has a Write function for"`
	Times map[string]bool `desc:"map of times that this item has a Write function for"`
}

func (item *Item) WriteFunc(mode, time string) (WriteFunc, bool) {
	val, ok := item.Write[etime.ScopeStr(mode, time)]
	return val, ok
}

// SetWriteFuncAll sets the Write function for all existing Modes and Times
// Can be used to replace a Write func after the fact.
func (item *Item) SetWriteFuncAll(theFunc WriteFunc) {
	for mode := range item.Modes {
		for time := range item.Times {
			item.Write[etime.ScopeStr(mode, time)] = theFunc
		}
	}
}

// SetWriteFuncOver sets the Write function over range of modes and times
func (item *Item) SetWriteFuncOver(modes []etime.Modes, times []etime.Times, theFunc WriteFunc) {
	for _, mode := range modes {
		for _, time := range times {
			item.Write[etime.Scope(mode, time)] = theFunc
		}
	}
}

// SetWriteFunc sets Write function for one mode, time
func (item *Item) SetWriteFunc(mode etime.Modes, time etime.Times, theFunc WriteFunc) {
	item.SetWriteFuncOver([]etime.Modes{mode}, []etime.Times{time}, theFunc)
}

// SetEachScopeKey updates the Write map so that it only contains entries
// for a unique Mode,Time pair, where multiple modes and times may have
// originally been specified.
func (item *Item) SetEachScopeKey() {
	newWrite := WriteMap{}
	doReplace := false
	for sk, c := range item.Write {
		modes, times := sk.ModesAndTimes()
		if len(modes) > 1 || len(times) > 1 {
			doReplace = true
			for _, m := range modes {
				for _, t := range times {
					newWrite[etime.ScopeStr(m, t)] = c
				}
			}
		} else {
			newWrite[sk] = c
		}
	}
	if doReplace {
		item.Write = newWrite
	}
}

// CompileScopes compiles maps of modes and times where this item appears.
// Based on the final updated Write map
func (item *Item) CompileScopes() {
	item.Modes = make(map[string]bool)
	item.Times = make(map[string]bool)
	for scope, _ := range item.Write {
		modes, times := scope.ModesAndTimes()
		for _, mode := range modes {
			item.Modes[mode] = true
		}
		for _, time := range times {
			item.Times[time] = true
		}
	}
}

func (item *Item) HasMode(mode etime.Modes) bool {
	_, has := item.Modes[mode.String()]
	return has
}

func (item *Item) HasTime(time etime.Times) bool {
	_, has := item.Times[time.String()]
	return has
}
