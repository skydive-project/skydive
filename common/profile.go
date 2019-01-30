/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package common

import (
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"
)

// Profile start profiling loop
func Profile() {
	cpu := make(chan os.Signal, 1)
	signal.Notify(cpu, syscall.SIGUSR1)

	memory := make(chan os.Signal, 1)
	signal.Notify(memory, syscall.SIGUSR2)

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
			f.Close()
			log.Println("CPU profiling Stopped")
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
		}
	}
}
