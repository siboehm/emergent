// Copyright (c) 2022, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package elog

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/emer/emergent/emer"
	"github.com/emer/emergent/estats"
	"github.com/emer/emergent/etime"
	"github.com/emer/empi/empi"
	"github.com/emer/empi/mpi"
	"github.com/emer/etable/etable"
)

// LogPrec is precision for saving float values in logs
const LogPrec = 4

// LogDir is a directory that is prefixed for saving log files
var LogDir = ""

// Logs contains all logging state and API for doing logging.
// do AddItem to add any number of items, at different eval mode, time scopes.
// Each Item has its own Write functions, at each scope as neeeded.
// Then call CreateTables to generate log Tables from those items.
// Call Log with a scope to add a new row of data to the log
// and ResetLog to reset the log to empty.
type Logs struct {
	Tables     map[etime.ScopeKey]*LogTable `desc:"Tables storing log data, auto-generated from Items."`
	MiscTables map[string]*etable.Table     `desc:"holds additional tables not computed from items -- e.g., aggregation results, intermediate computations, etc"`

	Items      []*Item          `view:"-" desc:"A list of the items that should be logged. Each item should describe one column that you want to log, and how.  Order in list determines order in logs."`
	Context    Context          `view:"-" desc:"context information passed to logging Write functions -- has all the information needed to compute and write log values -- is updated for each item in turn"`
	Modes      map[string]bool  `view:"-" desc:"All the eval modes that appear in any of the items of this log."`
	Times      map[string]bool  `view:"-" desc:"All the timescales that appear in any of the items of this log."`
	ItemIdxMap map[string]int   `view:"-" desc:"map of item indexes by name, for rapid access to items if they need to be modified after adding."`
	TableOrder []etime.ScopeKey `view:"-" desc:"sorted order of table scopes"`
}

// AddItem adds an item to the list.  The items are stored in the order
// they are added, and this order is used for calling the item Write
// functions, so you can rely on that ordering for any sequential
// dependencies across items (e.g., in using intermediate computed values).
// Note: item names must be unique -- use different scopes for Write functions
// where needed.
func (lg *Logs) AddItem(item *Item) *Item {
	lg.Items = append(lg.Items, item)
	if lg.ItemIdxMap == nil {
		lg.ItemIdxMap = make(map[string]int)
	}
	// note: we're not really in a position to track errors in a big list of
	// AddItem statements, so don't bother with error return
	if _, has := lg.ItemIdxMap[item.Name]; has {
		log.Printf("elog.AddItem Warning: item name repeated: %s -- item names must be unique -- use different scopes in their Write functions instead of adding multiple entries\n", item.Name)
	}
	lg.ItemIdxMap[item.Name] = len(lg.Items) - 1
	return item
}

// ItemByName returns item by given name, false if not found
func (lg *Logs) ItemByName(name string) (*Item, bool) {
	idx, has := lg.ItemIdxMap[name]
	if !has {
		return nil, false
	}
	itm := lg.Items[idx]
	return itm, true
}

// SetContext sets the Context for logging Write functions
// to give general access to the stats and network
func (lg *Logs) SetContext(stats *estats.Stats, net emer.Network) {
	lg.Context.Logs = lg
	lg.Context.Stats = stats
	lg.Context.Net = net
}

// Table returns the table for given mode, time
func (lg *Logs) Table(mode etime.Modes, time etime.Times) *etable.Table {
	sk := etime.Scope(mode, time)
	tb, ok := lg.Tables[sk]
	if !ok {
		// log.Printf("Table for scope not found: %s\n", sk)
		return nil
	}
	return tb.Table
}

// TableScope returns the table for given etime.ScopeKey
func (lg *Logs) TableScope(sk etime.ScopeKey) *etable.Table {
	tb, ok := lg.Tables[sk]
	if !ok {
		// log.Printf("Table for scope not found: %s\n", sk)
		return nil
	}
	return tb.Table
}

// MiscTable gets a miscellaneous table, e.g., for misc analysis.
// If it doesn't exist, one is created.
func (lg *Logs) MiscTable(name string) *etable.Table {
	dt, has := lg.MiscTables[name]
	if has {
		return dt
	}
	dt = &etable.Table{}
	lg.MiscTables[name] = dt
	return dt
}

// IdxView returns the Index View of a log table for a given mode, time
// This is used for data aggregation functions over the entire table.
// It should not be altered (don't Filter!) and always shows the whole table.
// See NamedIdxView for custom index views.
func (lg *Logs) IdxView(mode etime.Modes, time etime.Times) *etable.IdxView {
	return lg.IdxViewScope(etime.Scope(mode, time))
}

// IdxViewScope returns the Index View of a log table for given etime.ScopeKey
// This is used for data aggregation functions over the entire table.
// This view should not be altered and always shows the whole table.
// See NamedIdxView for custom index views.
func (lg *Logs) IdxViewScope(sk etime.ScopeKey) *etable.IdxView {
	lt := lg.Tables[sk]
	return lt.GetIdxView()
}

// NamedIdxView returns a named Index View of a log table for a given mode, time.
// This is used for additional data aggregation, filtering etc.
// When accessing the first time during writing a new row of the log,
// it automatically shows a view of the entire table and returns true for 2nd arg.
// You can then filter, sort, etc as needed.  Subsequent calls within same row Write will
// return the last filtered view, and false for 2nd arg -- can then just reuse view.
func (lg *Logs) NamedIdxView(mode etime.Modes, time etime.Times, name string) (*etable.IdxView, bool) {
	return lg.NamedIdxViewScope(etime.Scope(mode, time), name)
}

// NamedIdxView returns a named Index View of a log table for a given mode, time.
// This is used for additional data aggregation, filtering etc.
// When accessing the first time during writing a new row of the log,
// it automatically shows a view of the entire table and returns true for 2nd arg.
// You can then filter, sort, etc as needed.  Subsequent calls within same row Write will
// return the last filtered view, and false for 2nd arg -- can then just reuse view.
func (lg *Logs) NamedIdxViewScope(sk etime.ScopeKey, name string) (*etable.IdxView, bool) {
	lt := lg.Tables[sk]
	return lt.NamedIdxView(name)
}

// TableDetails returns the LogTable record of associated info for given table
func (lg *Logs) TableDetails(mode etime.Modes, time etime.Times) *LogTable {
	return lg.Tables[etime.Scope(mode, time)]
}

// TableDetailsScope returns the LogTable record of associated info for given table
func (lg *Logs) TableDetailsScope(sk etime.ScopeKey) *LogTable {
	return lg.Tables[sk]
}

// SetMeta sets table meta data for given scope mode, time.
func (lg *Logs) SetMeta(mode etime.Modes, time etime.Times, key, val string) {
	lg.SetMetaScope(etime.Scope(mode, time), key, val)
}

// SetMetaScope sets table meta data for given scope
func (lg *Logs) SetMetaScope(sk etime.ScopeKey, key, val string) {
	lt, has := lg.Tables[sk]
	if !has {
		return
	}
	lt.Meta[key] = val
}

// NoPlot sets meta data to not plot for given scope mode, time.
// Typically all combinations of mode and time end up being
// generated, so you have to turn off plotting of cases not used.
func (lg *Logs) NoPlot(mode etime.Modes, time ...etime.Times) {
	for _, tm := range time {
		lg.NoPlotScope(etime.Scope(mode, tm))
	}
}

// NoPlotScope sets meta data to not plot for given scope mode, time.
// Typically all combinations of mode and time end up being
// generated, so you have to turn off plotting of cases not used.
func (lg *Logs) NoPlotScope(sk etime.ScopeKey) {
	lg.SetMetaScope(sk, "Plot", "false")
}

// CreateTables creates the log tables based on all the specified log items
// It first calls ProcessItems to instantiate specific scopes.
func (lg *Logs) CreateTables() error {
	lg.ProcessItems()
	tables := make(map[etime.ScopeKey]*LogTable)
	tableOrder := make([]etime.ScopeKey, 0) //initial size
	var err error
	for _, item := range lg.Items {
		for scope, _ := range item.Write {
			_, has := tables[scope]
			modes, times := scope.ModesAndTimes()
			if len(modes) != 1 || len(times) != 1 {
				err = fmt.Errorf("Unexpected too long modes or times in: " + string(scope))
				log.Println(err) // actually print the err
			}
			if !has {
				dt := lg.NewTable(modes[0], times[0])
				tables[scope] = NewLogTable(dt)
				tableOrder = append(tableOrder, scope)
				if modes[0] == "Analyze" || modes[0] == "Validate" || modes[0] == "Debug" {
					tables[scope].Meta["Plot"] = "false" // don't plot by default
				}
			}
		}
	}
	lg.Tables = tables
	lg.TableOrder = etime.SortScopes(tableOrder)
	lg.MiscTables = make(map[string]*etable.Table)

	return err
}

// Log performs logging for given mode, time.
// Adds a new row and Writes all the items.
// and saves data to file if open.
func (lg *Logs) Log(mode etime.Modes, time etime.Times) *etable.Table {
	sk := etime.Scope(mode, time)
	lt := lg.Tables[sk]
	return lg.LogRow(mode, time, lt.Table.Rows)
}

// LogScope performs logging for given etime.ScopeKey
// Adds a new row and Writes all the items.
// and saves data to file if open.
func (lg *Logs) LogScope(sk etime.ScopeKey) *etable.Table {
	lt := lg.Tables[sk]
	return lg.LogRowScope(sk, lt.Table.Rows)
}

// LogRow performs logging for given mode, time, at given row.
// Saves data to file if open.
func (lg *Logs) LogRow(mode etime.Modes, time etime.Times, row int) *etable.Table {
	return lg.LogRowScope(etime.Scope(mode, time), row)
}

// LogRowScope performs logging for given etime.ScopeKey, at given row.
// Saves data to file if open.
func (lg *Logs) LogRowScope(sk etime.ScopeKey, row int) *etable.Table {
	lt := lg.Tables[sk]
	dt := lt.Table
	if row < 0 {
		row = dt.Rows
	}
	if dt.Rows <= row {
		dt.SetNumRows(row + 1)
	}
	lg.WriteItems(sk, row)
	lt.ResetIdxViews() // dirty that so it is regenerated later when needed
	lg.WriteLastRowToFile(lt)
	return dt
}

// ResetLog resets the log for given mode, time, at given row.
// by setting number of rows = 0
// The IdxViews are reset too.
func (lg *Logs) ResetLog(mode etime.Modes, time etime.Times) {
	sk := etime.Scope(mode, time)
	lt, ok := lg.Tables[sk]
	if !ok {
		return
	}
	dt := lt.Table
	dt.SetNumRows(0)
	lt.ResetIdxViews()
}

// MPIGatherTableRows calls empi.GatherTableRows on the given log table
// using an "MPI" suffixed MiscTable that is then switched out with the main table,
// so that any subsequent aggregation etc operates as usual on the full set of data.
// IMPORTANT: this switch means that the number of rows in the table MUST be reset
// back to either 0 (e.g., ResetLog) or the target number of rows, after the table
// is used, otherwise it will grow exponentially!
func (lg *Logs) MPIGatherTableRows(mode etime.Modes, time etime.Times, comm *mpi.Comm) {
	sk := etime.Scope(mode, time)
	lt := lg.Tables[sk]
	dt := lt.Table
	skm := string(sk + "MPI")
	mt, has := lg.MiscTables[skm]
	if !has {
		mt = &etable.Table{}
	}
	empi.GatherTableRows(mt, dt, comm)
	lt.Table = mt
	lg.MiscTables[skm] = dt // note: actual underlying tables are always being swapped
	lt.ResetIdxViews()
}

// SetLogFile sets the log filename for given scope
func (lg *Logs) SetLogFile(mode etime.Modes, time etime.Times, fnm string) {
	lt := lg.TableDetails(mode, time)
	if LogDir != "" {
		fnm = filepath.Join(LogDir, fnm)
	}
	var err error
	lt.File, err = os.Create(fnm)
	if err != nil {
		log.Println(err)
		lt.File = nil
	} else {
		fmt.Printf("Saving log to: %s\n", fnm)
	}
}

// CloseLogFiles closes all open log files
func (lg *Logs) CloseLogFiles() {
	for _, lt := range lg.Tables {
		if lt.File != nil {
			lt.File.Close()
			lt.File = nil
		}
	}
}

///////////////////////////////////////////////////////////////////////////
//   Internal infrastructure below, main user API above

// WriteItems calls all item Write functions within given scope
// providing the relevant Context for the function.
// Items are processed in the order added, to enable sequential
// dependencies to be used.
func (lg *Logs) WriteItems(sk etime.ScopeKey, row int) {
	lg.Context.SetTable(sk, lg.Tables[sk], row)
	for _, item := range lg.Items {
		fun, ok := item.Write[sk]
		if ok {
			lg.Context.Item = item
			fun(&lg.Context)
		}
	}
}

// WriteLastRowToFile writes the last row of table to file, if File != nil
func (lg *Logs) WriteLastRowToFile(lt *LogTable) {
	if lt.File == nil {
		return
	}
	dt := lt.Table
	if !lt.WroteHeaders {
		dt.WriteCSVHeaders(lt.File, etable.Tab)
		lt.WroteHeaders = true
	}
	dt.WriteCSVRow(lt.File, dt.Rows-1, etable.Tab)
}

// ProcessItems is called in CreateTables, after all items have been added.
// It instantiates All scopes, and compiles multi-list scopes into
// single mode, item pairs
func (lg *Logs) ProcessItems() {
	lg.CompileAllScopes()
	for _, item := range lg.Items {
		lg.ItemBindAllScopes(item)
		item.SetEachScopeKey()
		item.CompileScopes()
	}
}

// CompileAllScopes gathers all the modes and times used across all items
func (lg *Logs) CompileAllScopes() {
	lg.Modes = make(map[string]bool)
	lg.Times = make(map[string]bool)
	for _, item := range lg.Items {
		for sk, _ := range item.Write {
			modes, times := sk.ModesAndTimes()
			for _, m := range modes {
				if m == "AllModes" || m == "NoEvalMode" {
					continue
				}
				lg.Modes[m] = true
			}
			for _, t := range times {
				if t == "AllTimes" || t == "NoTime" {
					continue
				}
				lg.Times[t] = true
			}
		}
	}
}

// ItemBindAllScopes translates the AllModes or AllTimes scopes into
// a concrete list of actual Modes and Times used across all items
func (lg *Logs) ItemBindAllScopes(item *Item) {
	newMap := WriteMap{}
	for sk, c := range item.Write {
		newsk := sk
		useAllModes := false
		useAllTimes := false
		modes, times := sk.ModesAndTimesMap()
		for m := range modes {
			if m == "AllModes" {
				useAllModes = true
			}
		}
		for t := range times {
			if t == "AllTimes" {
				useAllTimes = true
			}
		}
		if useAllModes && useAllTimes {
			newsk = etime.ScopesMap(lg.Modes, lg.Times)
		} else if useAllModes {
			newsk = etime.ScopesMap(lg.Modes, times)
		} else if useAllTimes {
			newsk = etime.ScopesMap(modes, lg.Times)
		}
		newMap[newsk] = c
	}
	item.Write = newMap
}

// NewTable returns a new table configured for given mode, time scope
func (lg *Logs) NewTable(mode, time string) *etable.Table {
	dt := &etable.Table{}
	dt.SetMetaData("name", mode+time+"Log")
	dt.SetMetaData("desc", "Record of performance over "+time+" for "+mode)
	dt.SetMetaData("read-only", "true")
	dt.SetMetaData("precision", strconv.Itoa(LogPrec))
	sch := etable.Schema{}
	for _, val := range lg.Items {
		// Write is the definive record for which timescales are logged.
		if _, ok := val.WriteFunc(mode, time); ok {
			sch = append(sch, etable.Column{val.Name, val.Type, val.CellShape, val.DimNames})
		}
	}
	dt.SetFromSchema(sch, 0)
	return dt
}
