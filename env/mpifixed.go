// Copyright (c) 2020, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package env

import (
	"fmt"
	"log"
	"math/rand"

	"github.com/emer/emergent/erand"
	"github.com/emer/empi/empi"
	"github.com/emer/etable/etable"
	"github.com/emer/etable/etensor"
)

// MPIFixedTable is an MPI-enabled version of the FixedTable, which is
// a basic Env that manages patterns from an etable.Table, with
// either sequential or permuted random ordering, and uses standard Trial / Epoch
// TimeScale counters to record progress and iterations through the table.
// It also records the outer loop of Run as provided by the model.
// It uses an IdxView indexed view of the Table, so a single shared table
// can be used across different environments, with each having its own unique view.
// The MPI version distributes trials across MPI procs, in the Order list.
// It is ESSENTIAL that the number of trials (rows) in Table is
// evenly divisible by number of MPI procs!
// If all nodes start with the same seed, it should remain synchronized.
type MPIFixedTable struct {
	Nm         string          `desc:"name of this environment"`
	Dsc        string          `desc:"description of this environment"`
	Table      *etable.IdxView `desc:"this is an indexed view of the table with the set of patterns to output -- the indexes are used for the *sequential* view so you can easily sort / split / filter the patterns to be presented using this view -- we then add the random permuted Order on top of those if !sequential"`
	Sequential bool            `desc:"present items from the table in sequential order (i.e., according to the indexed view on the Table)?  otherwise permuted random order"`
	Order      []int           `desc:"permuted order of items to present if not sequential -- updated every time through the list"`
	Run        Ctr             `view:"inline" desc:"current run of model as provided during Init"`
	Epoch      Ctr             `view:"inline" desc:"number of times through entire set of patterns"`
	Trial      Ctr             `view:"inline" desc:"current ordinal item in Table -- if Sequential then = row number in table, otherwise is index in Order list that then gives row number in Table"`
	TrialName  CurPrvString    `desc:"if Table has a Name column, this is the contents of that"`
	GroupName  CurPrvString    `desc:"if Table has a Group column, this is contents of that"`
	NameCol    string          `desc:"name of the Name column -- defaults to 'Name'"`
	GroupCol   string          `desc:"name of the Group column -- defaults to 'Group'"`
	TrialSt    int             `desc:"for MPI, trial we start each epoch on, as index into Order"`
	TrialEd    int             `desc:"for MPI, trial number we end each epoch before (i.e., when ctr gets to Ed, restarts)"`
}

func (ft *MPIFixedTable) Name() string { return ft.Nm }
func (ft *MPIFixedTable) Desc() string { return ft.Dsc }

func (ft *MPIFixedTable) Validate() error {
	if ft.Table == nil || ft.Table.Table == nil {
		return fmt.Errorf("MPIFixedTable: %v has no Table set", ft.Nm)
	}
	if ft.Table.Table.NumCols() == 0 {
		return fmt.Errorf("MPIFixedTable: %v Table has no columns -- Outputs will be invalid", ft.Nm)
	}
	return nil
}

func (ft *MPIFixedTable) Init(run int) {
	if ft.NameCol == "" {
		ft.NameCol = "Name"
	}
	if ft.GroupCol == "" {
		ft.GroupCol = "Group"
	}
	ft.Run.Scale = Run
	ft.Epoch.Scale = Epoch
	ft.Trial.Scale = Trial
	ft.Run.Init()
	ft.Epoch.Init()
	ft.Trial.Init()
	ft.Run.Cur = run
	ft.NewOrder()
	ft.Trial.Cur = ft.TrialSt - 1 // init state -- key so that first Step() = ft.TrialSt
}

// NewOrder sets a new random Order based on number of rows in the table.
func (ft *MPIFixedTable) NewOrder() {
	np := ft.Table.Len()
	ft.Order = rand.Perm(np) // always start with new one so random order is identical
	// and always maintain Order so random number usage is same regardless, and if
	// user switches between Sequential and random at any point, it all works..
	ft.TrialSt, ft.TrialEd, _ = empi.AllocN(np)
	ft.Trial.Max = ft.TrialEd
}

// PermuteOrder permutes the existing order table to get a new random sequence of inputs
// just calls: erand.PermuteInts(ft.Order)
func (ft *MPIFixedTable) PermuteOrder() {
	erand.PermuteInts(ft.Order)
}

// Row returns the current row number in table, based on Sequential / perumuted Order and
// already de-referenced through the IdxView's indexes to get the actual row in the table.
func (ft *MPIFixedTable) Row() int {
	if ft.Sequential {
		return ft.Table.Idxs[ft.Trial.Cur]
	}
	return ft.Table.Idxs[ft.Order[ft.Trial.Cur]]
}

func (ft *MPIFixedTable) SetTrialName() {
	if nms := ft.Table.Table.ColByName(ft.NameCol); nms != nil {
		rw := ft.Row()
		if rw >= 0 && rw < nms.Len() {
			ft.TrialName.Set(nms.StringVal1D(rw))
		}
	}
}

func (ft *MPIFixedTable) SetGroupName() {
	if nms := ft.Table.Table.ColByName(ft.GroupCol); nms != nil {
		rw := ft.Row()
		if rw >= 0 && rw < nms.Len() {
			ft.GroupName.Set(nms.StringVal1D(rw))
		}
	}
}

func (ft *MPIFixedTable) Step() bool {
	ft.Epoch.Same() // good idea to just reset all non-inner-most counters at start

	if ft.Trial.Incr() { // if true, hit max, reset to 0
		ft.Trial.Cur = ft.TrialSt // key to reset always to start
		ft.PermuteOrder()
		ft.Epoch.Incr()
	}
	ft.SetTrialName()
	ft.SetGroupName()
	return true
}

func (ft *MPIFixedTable) Counters() []TimeScales {
	return []TimeScales{Run, Epoch, Trial}
}

func (ft *MPIFixedTable) Counter(scale TimeScales) (cur, prv int, chg bool) {
	switch scale {
	case Run:
		return ft.Run.Query()
	case Epoch:
		return ft.Epoch.Query()
	case Trial:
		return ft.Trial.Query()
	}
	return -1, -1, false
}

func (ft *MPIFixedTable) States() Elements {
	els := Elements{}
	els.FromSchema(ft.Table.Table.Schema())
	return els
}

func (ft *MPIFixedTable) State(element string) etensor.Tensor {
	et, err := ft.Table.Table.CellTensorTry(element, ft.Row())
	if err != nil {
		log.Println(err)
	}
	return et
}

func (ft *MPIFixedTable) Actions() Elements {
	return nil
}

func (ft *MPIFixedTable) Action(element string, input etensor.Tensor) {
	// nop
}

// Compile-time check that implements Env interface
var _ Env = (*MPIFixedTable)(nil)
