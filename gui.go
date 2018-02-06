package main

import (
  "fmt"
  "github.com/seanpringle/go-sdl2/sdl"
  "github.com/seanpringle/gostuff/box"
  "github.com/seanpringle/gostuff/history"
  "github.com/seanpringle/gostuff/jobqueue"
  "github.com/seanpringle/gostuff/menu"
  "github.com/seanpringle/gostuff/text"
  "image"
  "image/color"
  "strings"
  "time"
  "unsafe"
)

var (
  Dark  color.Color = color.RGBA{100, 100, 100, 255}
  Light color.Color = color.RGBA{200, 200, 200, 255}
)

type guiAPI struct {
  job  func(func())
  jobs func()
  tick func()
  exit func()
  box  func() box.Box
  done func() bool
}

const (
  BackGround int = iota
  Document
  Layers
)

type sprite struct {
  rgba  *image.RGBA
  layer int
  src   box.Box
  dst   box.Box
  angle float64 // degrees
  cache bool
}

func boxSDL(self box.Box) sdl.Rect {
  return sdl.Rect{int32(self.X), int32(self.Y), int32(self.W), int32(self.H)}
}

func newGUI() *guiAPI {

  self := &guiAPI{}
  jobs := jobqueue.New()

  self.jobs = jobs.Run
  self.job = jobs.Job

  assert := func(err error) {
    if err != nil {
      panic(err)
    }
  }

  quit := false

  self.done = func() bool {
    return quit
  }

  var window *sdl.Window
  var renderer *sdl.Renderer
  var aMask uint32 = 0
  var rMask uint32 = 0
  var gMask uint32 = 0
  var bMask uint32 = 0

  cli := (*menu.Menu)(nil)
  hist := history.New()

  view := box.Box{0, 0, 800, 600}
  mouse := box.Box{0, 0, 1, 1}

  self.box = func() box.Box {
    return view
  }

  //move := func(x, y int) {
  //  view = box.Box{x, y, view.W, view.H}
  //}

  //bump := func(x, y int) {
  //  move(view.X+x, view.Y+y)
  //}

  resize := func(w, h int) {
    view = box.Box{view.X, view.Y, w, h}
  }

  command := func(cmd string) bool {
    doc.Save()
    fields := strings.Fields(cmd)

    if len(fields) == 2 && fields[0] == "load" {
      doc.Load(fields[1])
      return true
    }

    if len(fields) == 1 && fields[0] == "load" {
      doc.Load("")
      return true
    }

    if len(fields) == 1 && fields[0] == "save" {
      doc.Save()
      return true
    }

    if len(fields) == 2 && fields[0] == "save" {
      doc.ReSave(fields[1])
      return true
    }

    if len(fields) == 2 && fields[0] == "autocomplete" {
      doc.Insert(fields[1])
      return true
    }

    if len(fields) == 3 && fields[0] == "set" {
      doc.Set(fields[1], fields[2])
      return true
    }

    if len(fields) == 2 && fields[0] == "drop" {
      doc.Drop(fields[1])
      return true
    }

    return false
  }

  if sdl.BYTEORDER == sdl.BIG_ENDIAN {
    aMask = 0x000000ff
    bMask = 0x0000ff00
    gMask = 0x00ff0000
    rMask = 0xff000000
  } else {
    rMask = 0x000000ff
    gMask = 0x0000ff00
    bMask = 0x00ff0000
    aMask = 0xff000000
  }

  var getTexture func(*image.RGBA) *sdl.Texture
  var dropTexture func(*image.RGBA)

  {
    cache := make(map[*image.RGBA]*sdl.Texture)

    create := func(rgba *image.RGBA) *sdl.Texture {
      rect := rgba.Bounds()
      w := rect.Dx()
      h := rect.Dy()

      surface, err := sdl.CreateRGBSurfaceFrom(
        unsafe.Pointer(&rgba.Pix[0]),
        w,
        h,
        32,
        w*4,
        rMask,
        gMask,
        bMask,
        aMask,
      )
      assert(err)

      texture, err := renderer.CreateTextureFromSurface(surface)
      assert(err)

      surface.Free()
      return texture
    }

    getTexture = func(img *image.RGBA) *sdl.Texture {
      if _, ok := cache[img]; !ok {
        cache[img] = create(img)
      }
      return cache[img]
    }

    dropTexture = func(img *image.RGBA) {
      if _, ok := cache[img]; ok {
        cache[img].Destroy()
        delete(cache, img)
      }
    }
  }

  var fps uint64
  var fps_rgba *image.RGBA
  var fps_from time.Time
  var fps_tick uint64

  self.tick = func() {
    tick := tick
    self.jobs()

    sprites := make(chan *sprite, 1000)
    go func() {
      doc.draw(view, sprites)
      close(sprites)
    }()

    sdl.Do(func() {

      renderer.SetDrawColor(0, 0, 0, 0)
      renderer.Clear()

      textures := make([]*sdl.Texture, 0)
      srects := make([]*sdl.Rect, 0)
      drects := make([]*sdl.Rect, 0)
      angles := make([]float64, 0)
      uncache := make([]*image.RGBA, 0)

      layers := [Layers][]*sprite{}
      for i := BackGround; i < Layers; i++ {
        layers[i] = []*sprite{}
      }
      for s := range sprites {
        layers[s.layer] = append(layers[s.layer], s)
      }

      for i := BackGround; i < Layers; i++ {
        for _, s := range layers[i] {
          src := boxSDL(s.src)
          dst := boxSDL(s.dst)

          textures = append(textures, getTexture(s.rgba))
          srects = append(srects, &src)
          drects = append(drects, &dst)
          angles = append(angles, s.angle)
          if !s.cache {
            uncache = append(uncache, s.rgba)
          }
        }
      }

      if cli != nil {

        input := cli.Input()
        matches := cli.Matches()
        position := cli.Position()

        x := 0
        if len(input) > 0 {
          rgba := text.Draw(Light, 2.0, input)
          rect := rgba.Bounds()

          dst := boxSDL(box.Box{x, view.H - 50 + ((50 - rect.Dy()) / 2), rect.Dx(), rect.Dy()})

          textures = append(textures, getTexture(rgba))
          uncache = append(uncache, rgba)
          srects = append(srects, nil)
          drects = append(drects, &dst)
          angles = append(angles, 0.0)

          x += rect.Dx() + 100
        }

        for i := 0; i < len(matches) && x < view.W; i++ {
          match := matches[i]

          var rgba *image.RGBA

          if position == i {
            rgba = text.DrawCache(Light, 2.0, match)
          } else {
            rgba = text.DrawCache(Dark, 2.0, match)
          }

          rect := rgba.Bounds()

          dst := boxSDL(box.Box{x, view.H - 50 + ((50 - rect.Dy()) / 2), rect.Dx(), rect.Dy()})

          textures = append(textures, getTexture(rgba))
          srects = append(srects, nil)
          drects = append(drects, &dst)
          angles = append(angles, 0.0)

          x += rect.Dx() + 50
        }
      }

      if time.Since(fps_from) >= time.Second {
        n := uint64(tick) - fps_tick
        fps_from = time.Now()
        fps_tick = uint64(tick)
        if fps != n {
          fps = n
          fps_rgba = text.DrawCache(Dark, 2.0, fmt.Sprintf("%d", fps))
        }
      }

      if fps_rgba != nil {
        rgba := fps_rgba
        rect := rgba.Bounds()
        textures = append(textures, getTexture(rgba))
        srects = append(srects, nil)
        drects = append(drects, &sdl.Rect{int32(view.W) - int32(rect.Dx()), 0, int32(rect.Dx()), int32(rect.Dy())})
        angles = append(angles, 0.0)
      }

      renderer.CopyBatch(textures, srects, drects, angles)

      for _, img := range uncache {
        dropTexture(img)
      }

      renderer.Present()

      pressed := false

      mod := sdl.GetModState()
      shift := mod&sdl.KMOD_SHIFT != 0
      ctrl := mod&sdl.KMOD_CTRL != 0

      editKey := func(key sdl.Keycode) {
        doc.Insert(sdl.GetKeyName(key))
      }

      editKeyCase := func(key sdl.Keycode) {
        chr := sdl.GetKeyName(key)
        if !shift {
          chr = strings.ToLower(chr)
        }
        doc.Insert(chr)
      }

      editShiftPair := func(key sdl.Keycode, alt sdl.Keycode) {
        chr := sdl.GetKeyName(key)
        if shift {
          chr = sdl.GetKeyName(alt)
        }
        doc.Insert(chr)
      }

      docEditing := map[sdl.Keycode]func(){
        sdl.K_a: func() {
          editKeyCase(sdl.K_a)
        },
        sdl.K_b: func() {
          editKeyCase(sdl.K_b)
        },
        sdl.K_c: func() {
          editKeyCase(sdl.K_c)
        },
        sdl.K_d: func() {
          editKeyCase(sdl.K_d)
        },
        sdl.K_e: func() {
          editKeyCase(sdl.K_e)
        },
        sdl.K_f: func() {
          editKeyCase(sdl.K_f)
        },
        sdl.K_g: func() {
          editKeyCase(sdl.K_g)
        },
        sdl.K_h: func() {
          editKeyCase(sdl.K_h)
        },
        sdl.K_i: func() {
          editKeyCase(sdl.K_i)
        },
        sdl.K_j: func() {
          editKeyCase(sdl.K_j)
        },
        sdl.K_k: func() {
          editKeyCase(sdl.K_k)
        },
        sdl.K_l: func() {
          editKeyCase(sdl.K_l)
        },
        sdl.K_m: func() {
          editKeyCase(sdl.K_m)
        },
        sdl.K_n: func() {
          editKeyCase(sdl.K_n)
        },
        sdl.K_o: func() {
          editKeyCase(sdl.K_o)
        },
        sdl.K_p: func() {
          editKeyCase(sdl.K_p)
        },
        sdl.K_q: func() {
          editKeyCase(sdl.K_q)
        },
        sdl.K_r: func() {
          editKeyCase(sdl.K_r)
        },
        sdl.K_s: func() {
          editKeyCase(sdl.K_s)
        },
        sdl.K_t: func() {
          editKeyCase(sdl.K_t)
        },
        sdl.K_u: func() {
          editKeyCase(sdl.K_u)
        },
        sdl.K_v: func() {
          editKeyCase(sdl.K_v)
        },
        sdl.K_w: func() {
          editKeyCase(sdl.K_w)
        },
        sdl.K_x: func() {
          editKeyCase(sdl.K_x)
        },
        sdl.K_y: func() {
          editKeyCase(sdl.K_y)
        },

        sdl.K_1: func() {
          if shift {
            doc.Exclaim()
            return
          }
          editKey(sdl.K_1)
        },

        sdl.K_2: func() {
          editShiftPair(sdl.K_2, sdl.K_AT)
        },

        sdl.K_3: func() {
          if ctrl {
            doc.Heading()
            return
          }
          editShiftPair(sdl.K_3, sdl.K_HASH)
        },

        sdl.K_4: func() {
          if ctrl {
            doc.Variable()
            return
          }
          editShiftPair(sdl.K_4, sdl.K_DOLLAR)
        },

        sdl.K_5: func() {
          editShiftPair(sdl.K_5, sdl.K_PERCENT)
        },

        sdl.K_6: func() {
          if ctrl {
            doc.UCFirst()
            return
          }
          editShiftPair(sdl.K_6, sdl.K_CARET)
        },

        sdl.K_7: func() {
          editShiftPair(sdl.K_7, sdl.K_AMPERSAND)
        },

        sdl.K_8: func() {
          if ctrl {
            doc.Bullet()
            return
          }
          editShiftPair(sdl.K_8, sdl.K_ASTERISK)
        },

        sdl.K_9: func() {
          if shift {
            doc.Paren()
            return
          }
          editKey(sdl.K_9)
        },

        sdl.K_0: func() {
          if shift {
            doc.Paren()
            return
          }
          editKey(sdl.K_0)
        },

        sdl.K_ESCAPE: func() {
          cli = menu.New("", []string{"load", "save", "set", "drop", "autocomplete"})
          hist.Last()
        },

        sdl.K_TAB: func() {
          cli = menu.New("autocomplete", doc.WordList(7))
        },

        sdl.K_UP: func() {
          if shift {
            doc.ShiftUp()
            return
          }
          doc.Up()
        },

        sdl.K_DOWN: func() {
          if shift {
            doc.ShiftDown()
            return
          }
          doc.Down()
        },

        sdl.K_LEFT: func() {
          doc.Left()
        },

        sdl.K_RIGHT: func() {
          doc.Right()
        },

        sdl.K_RETURN: func() {
          doc.Right()
        },

        sdl.K_BACKSPACE: func() {
          doc.BackSpace()
        },

        sdl.K_DELETE: func() {
          doc.Delete()
        },

        sdl.K_PERIOD: func() {
          doc.Period()
        },

        sdl.K_COMMA: func() {
          doc.Comma()
        },

        sdl.K_QUOTE: func() {
          if shift {
            doc.DQuote()
            return
          }
          editKey(sdl.K_QUOTE)
        },

        sdl.K_HOME: func() {
          if ctrl {
            doc.Top()
            return
          }
          doc.Home()
        },

        sdl.K_END: func() {
          if ctrl {
            doc.Bottom()
            return
          }
          doc.End()
        },

        sdl.K_SEMICOLON: func() {
          if shift {
            doc.Colon()
            return
          }
          doc.SemiColon()
        },

        sdl.K_MINUS: func() {
          if ctrl {
            doc.Emphasis()
            return
          }
          doc.Hyphen()
        },

        sdl.K_SPACE: func() {
          doc.Space()
        },
      }

      cliKey := func(key sdl.Keycode) {
        chr := sdl.GetKeyName(key)
        cli.Ins(chr)
      }

      cliKeyCase := func(key sdl.Keycode) {
        chr := sdl.GetKeyName(key)
        if !shift {
          chr = strings.ToLower(chr)
        }
        cli.Ins(chr)
      }

      cliShiftPair := func(key sdl.Keycode, alt sdl.Keycode) {
        chr := sdl.GetKeyName(key)
        if shift {
          chr = sdl.GetKeyName(alt)
        }
        cli.Ins(chr)
      }

      cliEditing := map[sdl.Keycode]func(){
        sdl.K_a: func() {
          cliKeyCase(sdl.K_a)
        },
        sdl.K_b: func() {
          cliKeyCase(sdl.K_b)
        },
        sdl.K_c: func() {
          cliKeyCase(sdl.K_c)
        },
        sdl.K_d: func() {
          cliKeyCase(sdl.K_d)
        },
        sdl.K_e: func() {
          cliKeyCase(sdl.K_e)
        },
        sdl.K_f: func() {
          cliKeyCase(sdl.K_f)
        },
        sdl.K_g: func() {
          cliKeyCase(sdl.K_g)
        },
        sdl.K_h: func() {
          cliKeyCase(sdl.K_h)
        },
        sdl.K_i: func() {
          cliKeyCase(sdl.K_i)
        },
        sdl.K_j: func() {
          cliKeyCase(sdl.K_j)
        },
        sdl.K_k: func() {
          cliKeyCase(sdl.K_k)
        },
        sdl.K_l: func() {
          cliKeyCase(sdl.K_l)
        },
        sdl.K_m: func() {
          cliKeyCase(sdl.K_m)
        },
        sdl.K_n: func() {
          cliKeyCase(sdl.K_n)
        },
        sdl.K_o: func() {
          cliKeyCase(sdl.K_o)
        },
        sdl.K_p: func() {
          cliKeyCase(sdl.K_p)
        },
        sdl.K_q: func() {
          cliKeyCase(sdl.K_q)
        },
        sdl.K_r: func() {
          cliKeyCase(sdl.K_r)
        },
        sdl.K_s: func() {
          cliKeyCase(sdl.K_s)
        },
        sdl.K_t: func() {
          cliKeyCase(sdl.K_t)
        },
        sdl.K_u: func() {
          cliKeyCase(sdl.K_u)
        },
        sdl.K_v: func() {
          cliKeyCase(sdl.K_v)
        },
        sdl.K_w: func() {
          cliKeyCase(sdl.K_w)
        },
        sdl.K_x: func() {
          cliKeyCase(sdl.K_x)
        },
        sdl.K_y: func() {
          cliKeyCase(sdl.K_y)
        },
        sdl.K_1: func() {
          cliShiftPair(sdl.K_1, sdl.K_EXCLAIM)
        },
        sdl.K_2: func() {
          cliShiftPair(sdl.K_2, sdl.K_AT)
        },
        sdl.K_3: func() {
          cliShiftPair(sdl.K_3, sdl.K_HASH)
        },
        sdl.K_4: func() {
          cliShiftPair(sdl.K_4, sdl.K_DOLLAR)
        },
        sdl.K_5: func() {
          cliShiftPair(sdl.K_5, sdl.K_PERCENT)
        },
        sdl.K_6: func() {
          cliShiftPair(sdl.K_6, sdl.K_CARET)
        },
        sdl.K_7: func() {
          cliShiftPair(sdl.K_7, sdl.K_AMPERSAND)
        },
        sdl.K_8: func() {
          cliShiftPair(sdl.K_8, sdl.K_ASTERISK)
        },
        sdl.K_9: func() {
          cliShiftPair(sdl.K_9, sdl.K_LEFTPAREN)
        },
        sdl.K_0: func() {
          cliShiftPair(sdl.K_0, sdl.K_RIGHTPAREN)
        },
        sdl.K_SPACE: func() {
          cliKey(sdl.K_SPACE)
        },
        sdl.K_PERIOD: func() {
          cliShiftPair(sdl.K_PERIOD, sdl.K_GREATER)
        },
        sdl.K_COMMA: func() {
          cliShiftPair(sdl.K_COMMA, sdl.K_LESS)
        },

        sdl.K_ESCAPE: func() {
          cli = nil
        },

        sdl.K_UP: func() {
          cli.Set(hist.Prev())
        },

        sdl.K_DOWN: func() {
          cli.Set(hist.Next())
        },

        sdl.K_TAB: func() {
          cli.Choose()
        },

        sdl.K_RETURN: func() {
          hist.Add(cli.Input())
          command(cli.Input())
          cli = nil
        },

        sdl.K_BACKSPACE: func() {
          cli.Back()
        },

        sdl.K_LEFT: func() {
          cli.Left()
        },

        sdl.K_RIGHT: func() {
          cli.Right()
        },
      }

      for {

        ev := sdl.PollEvent()
        if ev == nil {
          break
        }

        switch ev.(type) {

        case *sdl.QuitEvent:
          doc.Save()
          quit = true
          return

        case *sdl.WindowEvent:
          wev := ev.(*sdl.WindowEvent)
          switch wev.Event {

          case sdl.WINDOWEVENT_SIZE_CHANGED:
            resize(int(wev.Data1), int(wev.Data2))
          }

        case *sdl.MouseMotionEvent:
          mev := ev.(*sdl.MouseMotionEvent)
          mouse.X = int(mev.X) + view.X - view.W/2
          mouse.Y = int(mev.Y) + view.Y - view.H/2

        case *sdl.KeyDownEvent:

          if pressed {
            continue
          }

          pressed = true

          if cli == nil {

            handle := docEditing[ev.(*sdl.KeyDownEvent).Keysym.Sym]

            if handle != nil {
              handle()
            }

          } else {

            handle := cliEditing[ev.(*sdl.KeyDownEvent).Keysym.Sym]

            if handle != nil {
              handle()
            }
          }
        }
      }
    })
  }

  self.exit = func() {
    window.Destroy()
    renderer.Destroy()
  }

  sdl.Do(func() {
    var err error

    window, err = sdl.CreateWindow(
      "rts",
      sdl.WINDOWPOS_UNDEFINED,
      sdl.WINDOWPOS_UNDEFINED,
      800,
      600,
      sdl.WINDOW_SHOWN|sdl.WINDOW_RESIZABLE,
    )
    assert(err)

    renderer, err = sdl.CreateRenderer(
      window,
      -1,
      sdl.RENDERER_ACCELERATED|sdl.RENDERER_PRESENTVSYNC,
    )
    assert(err)

    err = renderer.
      SetDrawBlendMode(sdl.BLENDMODE_BLEND)
    assert(err)
  })

  return self
}
