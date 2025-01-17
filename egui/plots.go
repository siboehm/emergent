// Copyright (c) 2022, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package egui

import (
	"fmt"
	"log"

	"github.com/emer/emergent/elog"
	"github.com/emer/emergent/etime"
	"github.com/emer/etable/eplot"
	"github.com/emer/etable/etview"
	"github.com/goki/gi/gi"
)

// AddPlots adds plots based on the unique tables we have, currently assumes they should always be plotted
func (gui *GUI) AddPlots(title string, lg *elog.Logs) {
	gui.Plots = make(map[etime.ScopeKey]*eplot.Plot2D)
	// for key, table := range Log.Tables {
	for _, key := range lg.TableOrder {
		modes, times := key.ModesAndTimes()
		time := times[0]
		mode := modes[0]
		lt := lg.Tables[key] // LogTable struct
		if doplot, has := lt.Meta["Plot"]; has {
			if doplot == "false" {
				continue
			}
		}

		plt := gui.TabView.AddNewTab(eplot.KiT_Plot2D, mode+" "+time+" Plot").(*eplot.Plot2D)
		gui.Plots[key] = plt
		plt.SetTable(lt.Table)
		plt.Params.FmMetaMap(lt.Meta)

		ConfigPlotFromLog(title, plt, lg, key)
	}
}

func ConfigPlotFromLog(title string, plt *eplot.Plot2D, lg *elog.Logs, key etime.ScopeKey) {
	_, times := key.ModesAndTimes()
	time := times[0]
	lt := lg.Tables[key] // LogTable struct

	for _, item := range lg.Items {
		_, ok := item.Write[key]
		if !ok {
			continue
		}
		cp := plt.SetColParams(item.Name, item.Plot, item.FixMin, item.Range.Min, item.FixMax, item.Range.Max)

		if item.Color != "" {
			cp.ColorName = gi.ColorName(item.Color)
		}
		cp.TensorIdx = item.TensorIdx
		cp.ErrCol = item.ErrCol

		plt.Params.Title = title + " " + time + " Plot"
		plt.Params.XAxisCol = time
		if xaxis, has := lt.Meta["XAxisCol"]; has {
			plt.Params.XAxisCol = xaxis
		}
		if legend, has := lt.Meta["LegendCol"]; has {
			plt.Params.LegendCol = legend
		}
	}
}

// Plot returns plot for mode, time scope
func (gui *GUI) Plot(mode etime.Modes, time etime.Times) *eplot.Plot2D {
	return gui.PlotScope(etime.Scope(mode, time))
}

// PlotScope returns plot for given scope
func (gui *GUI) PlotScope(scope etime.ScopeKey) *eplot.Plot2D {
	if !gui.Active {
		return nil
	}
	plot, ok := gui.Plots[scope]
	if !ok {
		// fmt.Printf("egui Plot not found for scope: %s\n", scope)
		return nil
	}
	return plot
}

// SetPlot stores given plot in Plots map
func (gui *GUI) SetPlot(scope etime.ScopeKey, plt *eplot.Plot2D) {
	if gui.Plots == nil {
		gui.Plots = make(map[etime.ScopeKey]*eplot.Plot2D)
	}
	gui.Plots[scope] = plt
}

// UpdatePlot updates plot for given mode, time scope
func (gui *GUI) UpdatePlot(mode etime.Modes, time etime.Times) *eplot.Plot2D {
	plot := gui.Plot(mode, time)
	if plot != nil {
		plot.GoUpdate()
	}
	return plot
}

// UpdatePlotScope updates plot at given scope
func (gui *GUI) UpdatePlotScope(scope etime.ScopeKey) *eplot.Plot2D {
	plot := gui.PlotScope(scope)
	if plot != nil {
		plot.GoUpdate()
	}
	return plot
}

// UpdateCyclePlot updates cycle plot for given mode.
// only updates every CycleUpdateInterval
func (gui *GUI) UpdateCyclePlot(mode etime.Modes, cycle int) *eplot.Plot2D {
	plot := gui.Plot(mode, etime.Cycle)
	if plot == nil {
		return plot
	}
	if (gui.CycleUpdateInterval > 0) && (cycle%gui.CycleUpdateInterval == 0) {
		plot.GoUpdate()
	}
	return plot
}

// AddTableView adds a table view of given log,
// typically particularly useful for Debug logs.
func (gui *GUI) AddTableView(lg *elog.Logs, mode etime.Modes, time etime.Times) {
	if gui.TableViews == nil {
		gui.TableViews = make(map[etime.ScopeKey]*etview.TableView)
	}

	key := etime.Scope(mode, time)
	lt, ok := lg.Tables[key]
	if !ok {
		log.Printf("ERROR: in egui.AddTableView, log: %s not found\n", key)
		return
	}

	tv := gui.TabView.AddNewTab(etview.KiT_TableView, mode.String()+" "+time.String()+" ").(*etview.TableView)
	gui.TableViews[key] = tv
	tv.SetTable(lt.Table, nil)
}

// TableView returns TableView for mode, time scope
func (gui *GUI) TableView(mode etime.Modes, time etime.Times) *etview.TableView {
	if !gui.Active {
		return nil
	}
	scope := etime.Scope(mode, time)
	tv, ok := gui.TableViews[scope]
	if !ok {
		fmt.Printf("egui TableView not found for scope: %s\n", scope)
		return nil
	}
	return tv
}

// UpdateTableView updates TableView for given mode, time scope
func (gui *GUI) UpdateTableView(mode etime.Modes, time etime.Times) *etview.TableView {
	tv := gui.TableView(mode, time)
	if tv != nil {
		tv.UpdateTable()
	}
	return tv
}
