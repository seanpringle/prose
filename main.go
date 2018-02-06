package main

import (
  "flag"
  "github.com/seanpringle/go-sdl2/sdl"
  "log"
  "math"
  "os"
  "runtime/pprof"
  "sync/atomic"
  "time"
)

type tock uint64

var (
  profile  *bool = flag.Bool("profile", false, "cpu profile")
  gui      *guiAPI
  doc      *docAPI
  sequence uint64 = 0
  tick     tock   = 0
  fps      uint   = 30
)

func note(arg ...interface{}) {
  log.Println(arg...)
}

func id() uint64 {
  return atomic.AddUint64(&sequence, 1)
}

func seconds(sec float64) tock {
  return tock(math.Max(1, sec*float64(fps)))
}

func ticks(n tock) float64 {
  return math.Max(1.0, float64(n)) / float64(fps)
}

func main() {

  flag.Parse()

  if *profile {
    file, err := os.Create("profile")
    if err != nil {
      panic(err)
    }
    pprof.StartCPUProfile(file)
    defer pprof.StopCPUProfile()
  }

  sdl.Main(func() {

    gui = newGUI()

    path := ""
    for _, arg := range os.Args[1:] {
      path = arg
    }

    doc = newDoc(path)

    ticker := time.NewTicker(time.Millisecond * time.Duration(1000/fps))

    for !gui.done() {
      <-ticker.C
      tick++
      doc.tick()
      gui.tick()
    }

    ticker.Stop()
    doc.exit()
    gui.exit()
  })
}
