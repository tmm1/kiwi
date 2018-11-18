package where

// Helper for adding runtime info to the logger context

/* Copyright (c) 2016-2018, Alexander I.Grafov <grafov@gmail.com>
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
	"runtime"
	"strconv"
	"strings"

	"github.com/grafov/kiwi"
)

const (
	// Names that defines the that parts of runtime information should
	// be passed.
	File = 1
	Func = 2
	Line = 4

	stackJump = 2
)

// What adds runtime information to the logger context. Remember that
// it returns a slice of pairs so add it this way:
//
// log.Add(where.What(where.Filename, where.Func, where.Line)...)
func What(parts int) []*kiwi.Pair {
	var (
		pairs []*kiwi.Pair
		skip  = stackJump
	)
start:
	pc, file, _, _ := runtime.Caller(skip)

	if parts&Line > 0 {
		pairs = []*kiwi.Pair{{
			Key: "lineno",
			Eval: func() string {
				_, _, line, _ := runtime.Caller(skip)
				return strconv.Itoa(line)
			},
			Type: kiwi.IntegerVal}}
	}
	if parts&File > 0 {
		pairs = append(pairs, &kiwi.Pair{
			Key:  "file",
			Val:  file,
			Type: kiwi.StringVal,
		})
	}
	function := runtime.FuncForPC(pc).Name()
	if parts&Func > 0 {
		pairs = append(pairs, &kiwi.Pair{
			Key:  "function",
			Val:  function,
			Type: kiwi.StringVal,
		})
	}
	if strings.LastIndex(function, "grafov/kiwi.") != -1 {
		skip++
		goto start
	}
	return pairs
}