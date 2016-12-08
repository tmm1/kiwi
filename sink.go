package kiwi

// This file consists of Sink related structures and functions.
// Outputs accepts incoming log records from Loggers, check them with filters
// and write to output streams if checks passed.

/* Copyright (c) 2016, Alexander I.Grafov aka Axel
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

* Redistributions of source code must retain the above copyright notice, this
  list of conditions and the following disclaimer.

* Redistributions in binary form must reproduce the above copyright notice,
  this list of conditions and the following disclaimer in the documentation
  and/or other materials provided with the distribution.

* Neither the name of kvlog nor the names of its
  contributors may be used to endorse or promote products derived from
  this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

ॐ तारे तुत्तारे तुरे स्व */

import (
	"io"
	"sync"
	"time"
)

// Sinks accepts records through the chanels.
// Each sink has its own channel.
var collector struct {
	sync.RWMutex
	Sinks []*Sink
}

type (
	// Sink used for filtering incoming log records from all logger instances
	// and decides how to filter them. Each output wraps its own io.Writer.
	// Sink methods are safe for concurrent usage.
	Sink struct {
		id     uint
		In     chan *box
		writer io.Writer
		format Formatter

		sync.RWMutex
		closed          bool
		paused          bool
		positiveFilters map[string]Filter
		negativeFilters map[string]Filter
		hiddenKeys      map[string]bool
	}
	box struct {
		Record *[]pair
		Group  *sync.WaitGroup
	}
)

// SinkTo creates a new sink for an arbitrary number of loggers.
// There are any number of sinks may be created for saving incoming log
// records to different places.
// The sink requires explicit start with Start() before usage.
// That allows firstly setup filters before sink will really accept any records.
func SinkTo(w io.Writer, fn Formatter) *Sink {
	collector.RLock()
	for i, sink := range collector.Sinks {
		if sink.writer == w {
			collector.Sinks[i].format = fn
			collector.RUnlock()
			return collector.Sinks[i]
		}
	}
	collector.RUnlock()
	sink := &Sink{
		format:          fn,
		paused:          true,
		In:              make(chan *box, 16),
		writer:          w,
		positiveFilters: make(map[string]Filter),
		negativeFilters: make(map[string]Filter),
		hiddenKeys:      make(map[string]bool),
	}
	collector.Lock()
	sink.id = uint(len(collector.Sinks))
	collector.Sinks = append(collector.Sinks, sink)
	go processOutput(sink)
	collector.Unlock()
	return sink
}

// WithKey sets restriction for records output.
// Only the records WITH any of the keys will be passed to output.
func (s *Sink) WithKey(keys ...string) *Sink {
	s.Lock()
	if !s.closed {
		for _, key := range keys {
			s.positiveFilters[key] = &keyFilter{}
			delete(s.negativeFilters, key)
		}
	}
	s.Unlock()
	return s
}

// WithoutKey sets restriction for records output.
// Only the records WITHOUT any of the keys will be passed to output.
func (s *Sink) WithoutKey(keys ...string) *Sink {
	s.Lock()
	if !s.closed {
		for _, key := range keys {
			s.negativeFilters[key] = &keyFilter{}
			delete(s.positiveFilters, key)
		}
	}
	s.Unlock()
	return s
}

// WithValue sets restriction for records output.
// A record passed to output if the key equal one of any of the listed values.
func (s *Sink) WithValue(key string, vals ...string) *Sink {
	if len(vals) == 0 {
		return s.WithKey(key)
	}
	s.Lock()
	if !s.closed {
		s.positiveFilters[key] = &valsFilter{Vals: vals}
		delete(s.negativeFilters, key)
	}
	s.Unlock()
	return s
}

// WithoutValue sets restriction for records output.
func (s *Sink) WithoutValue(key string, vals ...string) *Sink {
	if len(vals) == 0 {
		return s.WithoutKey(key)
	}
	s.Lock()
	if !s.closed {
		s.negativeFilters[key] = &valsFilter{Vals: vals}
		delete(s.positiveFilters, key)
	}
	s.Unlock()
	return s
}

// WithInt64Range sets restriction for records output.
func (s *Sink) WithInt64Range(key string, from, to int64) *Sink {
	s.Lock()
	if !s.closed {
		delete(s.negativeFilters, key)
		s.positiveFilters[key] = &int64RangeFilter{From: from, To: to}
	}
	s.Unlock()
	return s
}

// WithoutInt64Range sets restriction for records output.
func (s *Sink) WithoutInt64Range(key string, from, to int64) *Sink {
	s.Lock()
	if !s.closed {
		delete(s.positiveFilters, key)
		s.negativeFilters[key] = &int64RangeFilter{From: from, To: to}
	}
	s.Unlock()
	return s
}

// WithFloat64Range sets restriction for records output.
func (s *Sink) WithFloat64Range(key string, from, to float64) *Sink {
	s.Lock()
	if !s.closed {
		delete(s.negativeFilters, key)
		s.positiveFilters[key] = &float64RangeFilter{From: from, To: to}
	}
	s.Unlock()
	return s
}

// WithoutFloat64Range sets restriction for records output.
func (s *Sink) WithoutFloat64Range(key string, from, to float64) *Sink {
	s.Lock()
	if !s.closed {
		delete(s.positiveFilters, key)
		s.negativeFilters[key] = &float64RangeFilter{From: from, To: to}
	}
	s.Unlock()
	return s
}

// WithTimeRange sets restriction for records output.
func (s *Sink) WithTimeRange(key string, from, to time.Time) *Sink {
	s.Lock()
	if !s.closed {
		delete(s.negativeFilters, key)
		s.positiveFilters[key] = &timeRangeFilter{From: from, To: to}
	}
	s.Unlock()
	return s
}

// WithoutTimeRange sets restriction for records output.
func (s *Sink) WithoutTimeRange(key string, from, to time.Time) *Sink {
	s.Lock()
	if !s.closed {
		delete(s.positiveFilters, key)
		s.negativeFilters[key] = &timeRangeFilter{From: from, To: to}
	}
	s.Unlock()
	return s
}

// WithFilter setup custom filtering function for values for a specific key.
// Custom filter should realize Filter interface. All custom filters treated
// as positive filters. So if the filter returns true then it will be passed.
func (s *Sink) WithFilter(key string, customFilter Filter) *Sink {
	s.Lock()
	if !s.closed {
		s.positiveFilters[key] = customFilter
	}
	s.Unlock()
	return s
}

// Reset all filters for the keys for the output.
func (s *Sink) Reset(keys ...string) *Sink {
	s.Lock()
	if !s.closed {
		for _, key := range keys {
			delete(s.positiveFilters, key)
			delete(s.negativeFilters, key)
		}
	}
	s.Unlock()
	return s
}

// Hide keys from the output. Other keys in record will be displayed
// but not hidden keys.
func (s *Sink) Hide(keys ...string) *Sink {
	s.Lock()
	if !s.closed {
		for _, key := range keys {
			s.hiddenKeys[key] = true
		}
	}
	s.Unlock()
	return s
}

// Unhide previously hidden keys. They will be displayed in the output again.
func (s *Sink) Unhide(keys ...string) *Sink {
	s.Lock()
	if !s.closed {
		for _, key := range keys {
			delete(s.hiddenKeys, key)
		}
	}
	s.Unlock()
	return s
}

// Stop stops writing to the output.
func (s *Sink) Stop() *Sink {
	s.Lock()
	s.paused = true
	s.Unlock()
	return s
}

// Start writing to the output.
// After creation of a new sink it will paused and you need explicitly start it.
// It allows setup the filters before the sink will accepts any records.
func (s *Sink) Start() *Sink {
	s.Lock()
	s.paused = false
	s.Unlock()
	return s
}

// Close the sink. Flush all records that came before.
func (s *Sink) Close() {
	s.Lock()
	if !s.closed {
		collector.Lock()
		s.closed = true
		s.writer = nil
		collector.Sinks = append(collector.Sinks[0:s.id], collector.Sinks[s.id+1:]...)
		collector.Unlock()
	}
	s.Unlock()
}

// Flush waits that all previously sent to the output records worked.
func (s *Sink) Flush() *Sink {
	s.RLock()
	if !s.closed {
		var wg sync.WaitGroup
		wg.Add(1)
		s.In <- &box{nil, &wg}
		wg.Wait()
	}
	s.RUnlock()
	return s
}

func processOutput(s *Sink) {
	var (
		box *box
		ok  bool
	)
	for {
		box, ok = <-s.In
		if !ok {
			s.Lock()
			s.closed = true
			s.positiveFilters = nil
			s.negativeFilters = nil
			s.hiddenKeys = nil
			s.Unlock()
			return
		}
		s.RLock()
		if s.closed {
			if box.Group != nil {
				box.Group.Done()
			}
			s.RUnlock()
			return
		}
		// Flush detected.
		if box.Record == nil && box.Group != nil {
			box.Group.Done()
			s.RUnlock()
			continue
		}
		if s.paused {
			if box.Group != nil {
				box.Group.Done()
			}
			s.RUnlock()
			continue
		}
		var (
			filter Filter
		)
		for _, pair := range *box.Record {
			// Negative conditions have highest priority
			if filter, ok = s.negativeFilters[pair.Key]; ok {
				if filter.Check(pair.Key, pair.Val.Strv) {
					goto skipRecord
				}
			}
			// At last check for positive conditions
			if filter, ok = s.positiveFilters[pair.Key]; ok {
				if !filter.Check(pair.Key, pair.Val.Strv) {
					goto skipRecord
				}
			}
		}
		s.filterRecord(box.Record)
	skipRecord:
		box.Group.Done()
		s.RUnlock()
	}
}

func (s *Sink) filterRecord(record *[]pair) {
	s.format.Begin()
	for _, pair := range *record {
		if ok := s.hiddenKeys[pair.Key]; ok {
			continue
		}
		s.format.Pair(pair.Key, pair.Val.Strv, pair.Val.Quoted)
	}
	if s.writer != nil {
		s.writer.Write(s.format.Finish())
	}
}

func sinkRecord(rec []pair) {
	var wg sync.WaitGroup
	collector.RLock()
	for _, s := range collector.Sinks {
		//		s.RLock()
		if !s.paused && !s.closed {
			wg.Add(1)
			s.In <- &box{&rec, &wg}
		}
		//		s.RUnlock()
	}
	collector.RUnlock()
	wg.Wait()
	// TODO release record to syncpool here
}
