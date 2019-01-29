// +build prof

/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package main

import (
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"

	"github.com/skydive-project/skydive/cmd/skydive"
)

func profile(cpu chan os.Signal, memory chan os.Signal) {
	for {
		select {
		case <-cpu:
			f, err := os.Create("/tmp/skydive-cpu.prof")
			if err != nil {
				log.Fatal("could not create CPU profile: ", err)
			}
			if err := pprof.StartCPUProfile(f); err != nil {
				log.Fatal("could not start CPU profile: ", err)
			}
			log.Println("CPU profiling Started")

			<-cpu

			pprof.StopCPUProfile()
			log.Println("CPU profiling Stopped")

			return
		case <-memory:
			// Memory Profile
			runtime.GC()
			memProfile, err := os.Create("/tmp/skydive-memory.prof")
			if err != nil {
				log.Fatal(err)
			}
			if err := pprof.WriteHeapProfile(memProfile); err != nil {
				log.Fatal(err)
			}
			memProfile.Close()
			log.Println("Memory profiling written")

			return
		}
	}
}

func main() {
	cpu := make(chan os.Signal, 1)
	signal.Notify(cpu, syscall.SIGUSR1)

	memory := make(chan os.Signal, 1)
	signal.Notify(memory, syscall.SIGUSR2)

	go profile(cpu, memory)

	skydive.RootCmd.Execute()
}
