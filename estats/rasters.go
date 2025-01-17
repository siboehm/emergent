// Copyright (c) 2022, The Emergent Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package estats

import (
	"github.com/emer/emergent/emer"
	"github.com/emer/etable/etensor"
)

// ConfigRasters configures spike rasters for given maximum number of cycles
// and layer names.
func (st *Stats) ConfigRasters(net emer.Network, maxCyc int, layers []string) {
	st.Rasters = layers
	for _, lnm := range st.Rasters {
		ly := net.LayerByName(lnm)
		sr := st.F32Tensor("Raster_" + lnm)
		nu := len(ly.RepIdxs())
		if nu == 0 {
			nu = ly.Shape().Len()
		}
		sr.SetShape([]int{nu, maxCyc}, nil, []string{"Nrn", "Cyc"})
	}
}

// SetRasterCol sets column of given raster from data
func (st *Stats) SetRasterCol(sr, tsr *etensor.Float32, col int) {
	for ni, v := range tsr.Values {
		sr.Set([]int{ni, col}, v)
	}
}

// RasterRec records data from layers configured with ConfigRasters
// using variable name, for given cycle number (X axis index)
func (st *Stats) RasterRec(net emer.Network, cyc int, varNm string) {
	for _, lnm := range st.Rasters {
		tsr := st.SetLayerRepTensor(net, lnm, varNm)
		sr := st.F32Tensor("Raster_" + lnm)
		if sr.Dim(1) <= cyc {
			continue
		}
		st.SetRasterCol(sr, tsr, cyc)
	}
}
