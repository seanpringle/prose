package main

import (
  "bufio"
  "container/list"
  "fmt"
  "github.com/seanpringle/gostuff/box"
  "github.com/seanpringle/gostuff/workerpool"
  "io/ioutil"
  "log"
  "os"
  "sort"
  "strings"
)

type docAPI struct {
  list  *list.List
  node  *list.Element
  path  string
  words []string
  vars  map[string]string
}

func newDoc(path string) *docAPI {
  self := &docAPI{}
  self.vars = map[string]string{}
  self.Load(path)
  return self
}

func (self *docAPI) tick() {
  if tick%seconds(60) == 0 {
    self.Save()
  }
}

func (self *docAPI) exit() {

}

func (self *docAPI) prevNext() (*paraAPI, *paraAPI) {
  prev := (*paraAPI)(nil)
  next := (*paraAPI)(nil)

  if self.node.Prev() != nil {
    prev = self.node.Prev().Value.(*paraAPI)
  }

  if self.node.Next() != nil {
    next = self.node.Next().Value.(*paraAPI)
  }

  return prev, next
}

func (self *docAPI) check() {

  discard := []*list.Element{}
  for e := self.list.Front(); e != nil; e = e.Next() {
    para := e.Value.(*paraAPI)
    if e != self.node && para.IsEmpty() {
      discard = append(discard, e)
    }
  }
  for _, e := range discard {
    self.list.Remove(e)
  }

  if self.list.Len() == 0 {
    self.list.PushFront(newPara(self))
  }

  if self.node == nil {
    self.node = self.list.Front()
  }
}

func (self *docAPI) Paragraph() *paraAPI {
  return self.node.Value.(*paraAPI)
}

func (self *docAPI) draw(view box.Box, sprites chan *sprite) {

  paraSpacing := func(lineHeight int) int {
    return int(float64(lineHeight) * 1.5)
  }

  view = view.Grow(-300, -100)

  fpos := view.Translate(0, (view.H-self.Paragraph().Height())/2)
  cpos := self.Paragraph().draw(true, fpos, view, sprites)

  upos := fpos
  dpos := cpos

  for e := self.node.Prev(); e != nil; e = e.Prev() {
    para := e.Value.(*paraAPI)
    upos.X = view.X
    upos.Y -= para.Height() + paraSpacing(para.LineHeight())
    if upos.Y < view.Y+view.H {
      para.draw(false, upos, view, sprites)
    }
  }

  dpos.Y += paraSpacing(self.Paragraph().LineHeight())

  for e := self.node.Next(); e != nil; e = e.Next() {
    para := e.Value.(*paraAPI)
    dpos.X = view.X
    if dpos.Y < view.Y+view.H {
      para.draw(false, dpos, view, sprites)
      dpos.Y += para.Height() + paraSpacing(para.LineHeight())
    }
  }
}

func (self *docAPI) ReSave(path string) {
  self.path = path
  self.Save()
}

func (self *docAPI) Save() {

  lines := []string{}

  for key, val := range self.vars {
    lines = append(lines, fmt.Sprintf("variable %s %s", key, val))
  }

  for e := self.list.Front(); e != nil; e = e.Next() {
    para := e.Value.(*paraAPI)
    for _, line := range para.Export() {
      lines = append(lines, line)
    }
  }

  content := strings.Join(lines, "\n")
  ioutil.WriteFile(self.path, []byte(content), 0644)
}

func (self *docAPI) Load(path string) {

  if path == "" {
    path = "autosave.prose"
  }

  self.list = list.New()
  self.node = self.list.PushFront(newPara(self))
  self.path = path

  defer self.check()

  file, err := os.Open(path)
  if err != nil {
    return
  }
  defer file.Close()

  scanner := bufio.NewScanner(file)
  lines := []string{}
  for scanner.Scan() {
    if strings.HasPrefix(scanner.Text(), "variable") {
      fields := strings.Fields(scanner.Text())
      self.Set(fields[1], fields[2])
      continue
    }
    if strings.HasPrefix(scanner.Text(), "paragraph") {
      if len(lines) > 0 {
        self.Paragraph().Import(lines)
      }
      lines = []string{scanner.Text()}
      self.node = self.list.PushBack(newPara(self))
      continue
    }
    lines = append(lines, scanner.Text())
  }
  if len(lines) > 0 {
    self.Paragraph().Import(lines)
  }
  if err := scanner.Err(); err != nil {
    log.Fatal(err)
  }
}

func (self *docAPI) Up() bool {
  defer self.check()

  fpos := self.Paragraph().Word().pos
  if self.Paragraph().Up(fpos) {
    return true
  }

  if self.node.Prev() != nil {
    self.Paragraph().Clean()
    self.node = self.node.Prev()
    self.Paragraph().Bottom()
    self.Paragraph().Up(fpos)
    return true
  }
  if !self.Paragraph().IsEmpty() {
    self.Paragraph().Clean()
    self.node = self.list.InsertBefore(newPara(self), self.node)
    return true
  }
  return false
}

func (self *docAPI) Down() bool {
  defer self.check()

  fpos := self.Paragraph().Word().pos
  if self.Paragraph().Down(fpos) {
    return true
  }

  if self.node.Next() != nil {
    self.Paragraph().Clean()
    self.node = self.node.Next()
    self.Paragraph().Top()
    self.Paragraph().Down(fpos)
    return true
  }
  if !self.Paragraph().IsEmpty() {
    self.Paragraph().Clean()
    self.node = self.list.InsertAfter(newPara(self), self.node)
    return true
  }
  return false
}

func (self *docAPI) Left() bool {
  defer self.check()
  return self.Paragraph().Left()
}

func (self *docAPI) Right() bool {
  defer self.check()
  return self.Paragraph().Right()
}

func (self *docAPI) ShiftUp() bool {
  defer self.check()
  if self.node.Prev() != nil {
    self.list.MoveBefore(self.node, self.node.Prev())
    return true
  }
  return false
}

func (self *docAPI) ShiftDown() bool {
  defer self.check()
  if self.node.Next() != nil {
    self.list.MoveAfter(self.node, self.node.Next())
    return true
  }
  return false
}

func (self *docAPI) Return() {
  defer self.check()
  prev := self.Paragraph()
  self.node = self.list.InsertAfter(newPara(self), self.node)
  next := self.Paragraph()
  prev.Split(next)
  prev.Clean()
  next.Clean()
  next.Top()
}

func (self *docAPI) Insert(str string) {
  self.Paragraph().Insert(str)
}

func (self *docAPI) Space() {
  self.Paragraph().Space()
}

func (self *docAPI) BackSpace() {
  self.Paragraph().BackSpace()
}

func (self *docAPI) Delete() {
  defer self.check()
  if self.node.Next() != nil && self.Paragraph().IsEnd() {
    next := self.node.Next().Value.(*paraAPI)
    next.Top()
    next.Split(self.Paragraph())
    self.Paragraph().Right()
    return
  }
  self.Paragraph().Delete()
}

func (self *docAPI) DQuote() {
  self.Paragraph().DQuote()
}

func (self *docAPI) Period() {
  self.Paragraph().Period()
}

func (self *docAPI) Comma() {
  self.Paragraph().Comma()
}

func (self *docAPI) Exclaim() {
  self.Paragraph().Exclaim()
}

func (self *docAPI) Question() {
  self.Paragraph().Question()
}

func (self *docAPI) Hyphen() {
  self.Paragraph().Hyphen()
}

func (self *docAPI) Emphasis() {
  self.Paragraph().Emphasis()
}

func (self *docAPI) UCFirst() {
  self.Paragraph().UCFirst()
}

func (self *docAPI) Heading() {
  self.Paragraph().Heading()
}

func (self *docAPI) Bullet() {
  self.Paragraph().Bullet()
}

func (self *docAPI) Paren() {
  self.Paragraph().Paren()
}

func (self *docAPI) Colon() {
  self.Paragraph().Colon()
}

func (self *docAPI) SemiColon() {
  self.Paragraph().SemiColon()
}

func (self *docAPI) Home() {
  self.Paragraph().Home()
}

func (self *docAPI) End() {
  self.Paragraph().End()
}

func (self *docAPI) Top() {
  self.Paragraph().Top()
}

func (self *docAPI) Bottom() {
  self.Paragraph().Bottom()
}

func (self *docAPI) WordList(min int) []string {
  words := map[string]struct{}{}

  recv := workerpool.New(1)
  pool := workerpool.New(8)

  stream := make(chan string, 100)
  recv.Job(func() {
    for word := range stream {
      words[word] = struct{}{}
    }
  })

  for e := self.list.Front(); e != nil; e = e.Next() {
    para := e.Value.(*paraAPI)
    if !para.IsEmpty() {
      pool.Job(func() {
        for _, word := range para.WordList(min) {
          stream <- word
        }
      })
    }
  }

  pool.Wait()
  close(stream)
  recv.Wait()

  list := []string{}
  for word, _ := range words {
    list = append(list, word)
  }
  sort.Strings(list)

  return list
}

func (self *docAPI) Get(name string, def string) string {
  if val, ok := self.vars[name]; ok {
    return val
  }
  return def
}

func (self *docAPI) Set(name string, val string) {
  self.vars[name] = val
}

func (self *docAPI) Drop(name string) bool {
  if _, ok := self.vars[name]; ok {
    delete(self.vars, name)
    return true
  }
  return false
}

func (self *docAPI) Exists(name string) bool {
  _, ok := self.vars[name]
  return ok
}

func (self *docAPI) Variable() {
  self.Paragraph().Variable()
}
