package main

import (
  "container/list"
  "github.com/seanpringle/gostuff/box"
  "github.com/seanpringle/gostuff/text"
  "image"
  "image/color"
  "sort"
  "strings"
)

const (
  Heading int = iota
  Content
  Bullet
  Comment
  Quote
  Focus
  Highlight
)

var (
  fontSizes  map[int]float64
  fontColors map[int]color.RGBA
)

func init() {

  fontSizes = map[int]float64{
    Heading: 4.0,
    Content: 3.0,
  }
  fontSizes[Bullet] = fontSizes[Content]
  fontSizes[Comment] = fontSizes[Content]

  fontColors = map[int]color.RGBA{
    Focus:     color.RGBA{200, 200, 0, 255},
    Heading:   color.RGBA{255, 255, 255, 255},
    Content:   color.RGBA{200, 200, 200, 255},
    Comment:   color.RGBA{100, 100, 100, 255},
    Quote:     color.RGBA{150, 200, 150, 255},
    Highlight: color.RGBA{255, 255, 255, 255},
  }
  fontColors[Bullet] = fontColors[Content]
}

type paraAPI struct {
  doc    *docAPI
  list   *list.List
  node   *list.Element
  height int
  style  int
}

func newPara(doc *docAPI) *paraAPI {
  self := &paraAPI{}
  self.doc = doc
  self.list = list.New()
  self.node = self.list.PushFront(newWord(self))
  self.style = Content
  return self
}

func (self *paraAPI) check() {

  discard := []*list.Element{}
  for e := self.list.Front(); e != nil; e = e.Next() {
    if e != self.node && e.Value.(*wordAPI).Len() == 0 {
      discard = append(discard, e)
    }
  }
  for _, e := range discard {
    self.list.Remove(e)
  }

  if self.list.Len() == 0 {
    self.list.PushFront(newWord(self))
  }

  if self.node == nil {
    self.node = self.list.Front()
  }
}

func (self *paraAPI) IsEmpty() bool {
  return self.Len() == 0 || (self.Len() == 1 && self.Word().IsEmpty())
}

func (self *paraAPI) IsEnd() bool {
  return self.node.Next() == nil
}

func (self *paraAPI) Len() int {
  return self.list.Len()
}

func (self *paraAPI) Height() int {
  return self.height
}

func (self *paraAPI) LineHeight() int {
  rgba := text.DrawCache(Light, fontSizes[self.style], "Jj")
  return rgba.Bounds().Dy()
}

func (self *paraAPI) Word() *wordAPI {
  return self.node.Value.(*wordAPI)
}

func (self *paraAPI) Clean() {
  if self.Word().IsEmpty() {
    self.Left()
  }
  if self.Word().IsEmpty() {
    self.Right()
  }
  if self.Word().IsEmpty() {
    self.node = nil
  }
  self.check()
}

func (self *paraAPI) prevNext() (*wordAPI, *wordAPI) {
  prev := (*wordAPI)(nil)
  next := (*wordAPI)(nil)

  if self.node.Prev() != nil {
    prev = self.node.Prev().Value.(*wordAPI)
  }

  if self.node.Next() != nil {
    next = self.node.Next().Value.(*wordAPI)
  }

  return prev, next
}

func (self *paraAPI) draw(focus bool, pos box.Box, view box.Box, sprites chan *sprite) box.Box {

  x := pos.X
  y := pos.Y

  lineSpacing := func() int {
    return int(float64(self.LineHeight()) * 1.2)
  }

  emitText := func(rgba *image.RGBA) box.Box {

    rect := rgba.Bounds()
    if pos.X+rect.Dx() > view.X+view.W {
      pos.X = x
      pos.Y += lineSpacing()
    }

    dst := box.Box{pos.X, pos.Y, rect.Dx(), rect.Dy()}

    sprites <- &sprite{
      rgba:  rgba,
      layer: Document,
      src:   box.Box{0, 0, rect.Dx(), rect.Dy()},
      dst:   dst,
      cache: true,
    }
    pos = pos.Translate(rect.Dx(), 0)
    return dst
  }

  prev := (*wordAPI)(nil)
  next := (*wordAPI)(nil)

  if self.style == Bullet {
    emitText(text.DrawCache(fontColors[Bullet], fontSizes[self.style], "• "))
  }

  for e := self.list.Front(); e != nil; e = e.Next() {

    next = nil
    if e.Next() != nil {
      next = e.Next().Value.(*wordAPI)
    }

    word := e.Value.(*wordAPI)
    color := fontColors[self.style]

    if word.IsDQuote() {
      color = fontColors[Quote]
    }

    if word.IsParen() {
      color = fontColors[Comment]
    }

    if word.IsEmphasis() {
      color = fontColors[Highlight]
    }

    if focus && e == self.node {
      color = fontColors[Focus]
    }

    showVar := focus && word.IsVariable() && self.Word() == word
    wordStr := word.Format(prev, next, showVar)

    word.pos = emitText(text.DrawCache(color, fontSizes[self.style], wordStr))

    prev = word
  }

  self.height = pos.Y - y

  return pos
}

func (self *paraAPI) Export() []string {
  words := []string{}

  flags := []string{
    "paragraph",
  }

  if self.style == Heading {
    flags = append(flags, "heading")
  }

  if self.style == Bullet {
    flags = append(flags, "bullet")
  }

  words = append(words, strings.Join(flags, " "))

  prev := (*wordAPI)(nil)
  next := (*wordAPI)(nil)

  for e := self.list.Front(); e != nil; e = e.Next() {

    next = nil
    if e.Next() != nil {
      next = e.Next().Value.(*wordAPI)
    }

    word := e.Value.(*wordAPI)
    if !word.IsEmpty() {
      words = append(words, word.Export(prev, next))
    }

    prev = word
  }
  return words
}

func (self *paraAPI) Import(words []string) {
  defer self.Home()
  defer self.check()

  for _, line := range words {
    if strings.HasPrefix(line, "paragraph") {
      if strings.Contains(line, "heading") {
        self.style = Heading
      }
      if strings.Contains(line, "bullet") {
        self.style = Bullet
      }
      continue
    }
    self.node = self.list.PushBack(newWord(self))
    self.Word().Import(line)
  }
}

func (self *paraAPI) Top() {
  defer self.check()
  self.node = self.list.Front()
}

func (self *paraAPI) Bottom() {
  defer self.check()
  self.node = self.list.Back()
}

func (self *paraAPI) Up(fpos box.Box) bool {
  defer self.check()

  fpos = fpos.Grow(-fpos.W/3, 0)               // horizontal central third
  fpos = fpos.Translate(0, -self.LineHeight()) // prev line
  fpos = fpos.Extend(0, -self.LineHeight()*10) // intersect prev line

  for e := self.list.Back(); e != nil; e = e.Prev() {
    word := e.Value.(*wordAPI)
    if fpos.Intersects(word.pos) {
      self.node = e
      return true
    }
  }

  return false
}

func (self *paraAPI) Down(fpos box.Box) bool {
  defer self.check()

  fpos = fpos.Grow(-fpos.W/3, 0)              // horizontal central third
  fpos = fpos.Translate(0, self.LineHeight()) // next line
  fpos = fpos.Extend(0, self.LineHeight()*10) // intersect next line

  for e := self.list.Front(); e != nil; e = e.Next() {
    word := e.Value.(*wordAPI)
    if fpos.Intersects(word.pos) {
      self.node = e
      return true
    }
  }

  return false
}

func (self *paraAPI) Left() bool {
  defer self.check()
  if self.node.Prev() != nil {
    self.node = self.node.Prev()
    return true
  }
  if !self.Word().IsEmpty() {
    self.node = self.list.InsertBefore(newWord(self), self.node)
    return true
  }
  return false
}

func (self *paraAPI) Right() bool {
  defer self.check()
  if self.node.Next() != nil {
    self.node = self.node.Next()
    return true
  }
  if !self.Word().IsEmpty() {
    self.Space()
    return true
  }
  return false
}

func (self *paraAPI) Insert(str string) {
  self.Word().Insert(str)
}

func (self *paraAPI) Space() {
  defer self.check()
  self.node = self.list.InsertAfter(newWord(self), self.node)
  self.Word().Smart(self.prevNext())
}

func (self *paraAPI) BackSpace() {
  defer self.check()
  if !self.Word().BackSpace() {
    self.node = self.node.Prev()
  }
}

func (self *paraAPI) Delete() {
  defer self.check()
  self.Word().BackSpace()
  self.node = self.node.Next()
}

func (self *paraAPI) DQuote() {
  self.Word().DQuote()
}

func (self *paraAPI) Period() {
  self.Word().Period()
}

func (self *paraAPI) Comma() {
  self.Word().Comma()
}

func (self *paraAPI) Exclaim() {
  self.Word().Exclaim()
}

func (self *paraAPI) Question() {
  self.Word().Question()
}

func (self *paraAPI) Hyphen() {
  self.Word().Hyphen()
}

func (self *paraAPI) Paren() {
  self.Word().Paren()
}

func (self *paraAPI) Colon() {
  self.Word().Colon()
}

func (self *paraAPI) SemiColon() {
  self.Word().SemiColon()
}

func (self *paraAPI) UCFirst() {
  self.Word().UCFirst()
}

func (self *paraAPI) Heading() {
  if self.style != Heading {
    self.style = Heading
    return
  }
  self.style = Content
}

func (self *paraAPI) Bullet() {
  if self.style != Bullet {
    self.style = Bullet
    return
  }
  self.style = Content
}

func (self *paraAPI) Emphasis() {
  self.Word().Emphasis()
}

func (self *paraAPI) Home() {
  defer self.check()

  fpos := self.Word().pos

  for e := self.node; e != nil; e = e.Prev() {
    self.node = e
    if e.Prev() != nil {
      word := e.Prev().Value.(*wordAPI)
      if word.pos.Y < fpos.Y {
        break
      }
    }
  }
}

func (self *paraAPI) End() {
  defer self.check()

  fpos := self.Word().pos

  for e := self.node; e != nil; e = e.Next() {
    self.node = e
    if e.Next() != nil {
      word := e.Next().Value.(*wordAPI)
      if word.pos.Y > fpos.Y {
        break
      }
    }
  }
}

func (self *paraAPI) Variable() {
  self.Word().Variable()
}

func (self *paraAPI) WordList(min int) []string {
  words := map[string]struct{}{}
  for e := self.list.Front(); e != nil; e = e.Next() {
    word := e.Value.(*wordAPI)
    if !word.IsEmpty() && word.Len() >= min {
      words[word.text] = struct{}{}
    }
  }
  list := []string{}
  for word, _ := range words {
    list = append(list, word)
  }
  sort.Strings(list)
  return list
}

func (self *paraAPI) AddWord(word *wordAPI) {
  defer self.check()
  word.Reparent(self)
  self.node = self.list.InsertAfter(word, self.node)
}

func (self *paraAPI) Split(next *paraAPI) {
  defer self.check()
  for self.node != nil && !self.Word().IsEmpty() {
    next.AddWord(self.Word())
    node := self.node
    next := self.node.Next()
    self.list.Remove(node)
    self.node = next
  }
}
