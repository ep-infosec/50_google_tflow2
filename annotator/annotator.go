// Copyright 2017 Google Inc. All Rights Reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package annotator annotates flows with meta data from external sources
package annotator

import (
	"sync/atomic"

	"github.com/google/tflow2/annotator/bird"
	"github.com/google/tflow2/netflow"
	"github.com/google/tflow2/stats"
)

// Annotator represents an flow annotator
type Annotator struct {
	inputs        []chan *netflow.Flow
	output        chan *netflow.Flow
	aggregation   int64
	numWorkers    int
	bgpAugment    bool
	birdAnnotator *bird.Annotator
	debug int
}

// New creates a new `Annotator` instance
func New(inputs []chan *netflow.Flow, output chan *netflow.Flow, numWorkers int, aggregation int64, bgpAugment bool, birdSock string, birdSock6 string, debug int) *Annotator {
	a := &Annotator{
		inputs:      inputs,
		output:      output,
		aggregation: aggregation,
		numWorkers:  numWorkers,
		bgpAugment:  bgpAugment,
		debug:	debug,
	}
	if bgpAugment {
		a.birdAnnotator = bird.NewAnnotator(birdSock, birdSock6, debug)
	}
	a.Init()
	return a
}

// Init get's the annotation layer started, receives flows, annotates them, and carries them
// further to the database module
func (a *Annotator) Init() {
	for _, ch := range a.inputs {
		for i := 0; i < a.numWorkers; i++ {
			go func(ch chan *netflow.Flow) {
				for {
					// Read flow from netflow/IPFIX module
					fl := <-ch

					// Align timestamp on `aggrTime` raster
					fl.Timestamp = fl.Timestamp - (fl.Timestamp % a.aggregation)

					// Update global statstics
					atomic.AddUint64(&stats.GlobalStats.FlowBytes, fl.Size)
					atomic.AddUint64(&stats.GlobalStats.FlowPackets, uint64(fl.Packets))

					// Annotate flows with ASN and Prefix information from local BIRD (bird.nic.cz) instance
					if a.bgpAugment {
						a.birdAnnotator.Augment(fl)
					}

					// Send flow over to database module
					a.output <- fl
				}
			}(ch)
		}
	}
}
