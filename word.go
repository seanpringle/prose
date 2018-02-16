package main

import (
  "fmt"
  "github.com/seanpringle/gostuff/box"
  "strconv"
  "strings"
  "unicode"
)

const (
  DQuote uint64 = 1 << iota
  Period
  Exclaim
  Question
  Comma
  Ellipsis
  Hyphen
  Paren
  Colon
  SemiColon
  Emphasis
  Variable
)

type wordAPI struct {
  para  *paraAPI
  text  string
  flags uint64
  pos   box.Box
}

func newWord(para *paraAPI) *wordAPI {
  self := &wordAPI{}
  self.para = para
  return self
}

func (self *wordAPI) Is(flag uint64) bool {
  return (self.flags & flag) == flag
}

func (self *wordAPI) Set(flag uint64) {
  self.flags |= flag
}

func (self *wordAPI) Clr(flag uint64) {
  self.flags &^= flag
}

func (self *wordAPI) Toggle(flag uint64) bool {
  self.flags ^= flag
  return self.Is(flag)
}

func (self *wordAPI) IsEmpty() bool {
  return self.Len() == 0
}

func (self *wordAPI) Len() int {
  return len(self.text)
}

func (self *wordAPI) Text() string {
  return self.text
}

func (self *wordAPI) Display() string {
  if self.Is(Variable) {
    return self.para.doc.Get(self.text, self.text)
  }
  return self.text
}

func (self *wordAPI) Format(prev *wordAPI, next *wordAPI, showVar bool) string {

  str := self.Display()
  if showVar && self.IsVariable() {
    str = self.Text()
  }
  if len(str) == 0 {
    str = "_"
  }

  prefix := ""
  suffix := ""
  gap := " "

  if self.Is(Comma) {
    suffix = ","
  }

  if self.Is(Period) {
    suffix = "."
  }

  if self.Is(Ellipsis) {
    suffix = "…"
  }

  if self.Is(Exclaim) {
    suffix = "!"
  }

  if self.Is(Question) {
    suffix = "?"
  }

  if self.Is(Colon) {
    suffix = ":"
  }

  if self.Is(SemiColon) {
    suffix = ";"
  }

  if self.Is(Hyphen) {
    suffix = "-"
    gap = ""
  }

  if self.Is(DQuote) {
    if prev == nil || !prev.Is(DQuote) {
      prefix = "\""
    }
    if next == nil || !next.Is(DQuote) {
      suffix = suffix + "\""
    }
  }

  if self.Is(Paren) {
    if prev == nil || !prev.Is(Paren) {
      prefix = "("
    }
    if next == nil || !next.Is(Paren) {
      suffix = suffix + ")"
    }
  }

  return prefix + str + suffix + gap
}

func (self *wordAPI) Export(prev *wordAPI, next *wordAPI) string {
  return fmt.Sprintf("%v,%s",
    self.flags,
    self.text,
  )
}

func (self *wordAPI) Import(line string) {
  fields := strings.Split(line, ",")
  self.flags, _ = strconv.ParseUint(fields[0], 10, 64)
  self.text = strings.TrimSpace(fields[1])
}

func (self *wordAPI) Smart(prev *wordAPI, next *wordAPI) {
  if prev != nil && prev.Is(DQuote) {
    self.Set(DQuote)
  }
}

func (self *wordAPI) Insert(str string) {
  self.text = self.text + str
}

func (self *wordAPI) BackSpace() bool {
  if len(self.text) > 0 {
    self.text = ""
    return true
  }
  return false
}

func (self *wordAPI) DQuote() {
  self.Toggle(DQuote)
}

func (self *wordAPI) IsDQuote() bool {
  return self.Is(DQuote)
}

func (self *wordAPI) Paren() {
  self.Toggle(Paren)
}

func (self *wordAPI) IsParen() bool {
  return self.Is(Paren)
}

func (self *wordAPI) Emphasis() {
  self.Toggle(Emphasis)
}

func (self *wordAPI) IsEmphasis() bool {
  return self.Is(Emphasis)
}

func (self *wordAPI) Variable() {
  self.Toggle(Variable)
}

func (self *wordAPI) IsVariable() bool {
  return self.Is(Variable)
}

func (self *wordAPI) ClearPunct() {
}

func (self *wordAPI) TogglePunct(flag uint64) {
  if self.Toggle(flag) {
    self.Clr((Comma | Period | Ellipsis | Exclaim | Question | Hyphen | Colon | SemiColon) &^ flag)
  }
}

func (self *wordAPI) Period() {
  if self.Is(Period | Ellipsis) {
    self.Ellipsis()
    return
  }
  self.TogglePunct(Period)
}

func (self *wordAPI) Ellipsis() {
  self.TogglePunct(Ellipsis)
}

func (self *wordAPI) Comma() {
  self.TogglePunct(Comma)
}

func (self *wordAPI) Exclaim() {
  self.TogglePunct(Exclaim)
}

func (self *wordAPI) Question() {
  self.TogglePunct(Question)
}

func (self *wordAPI) Hyphen() {
  self.TogglePunct(Hyphen)
}

func (self *wordAPI) Colon() {
  self.TogglePunct(Colon)
}

func (self *wordAPI) SemiColon() {
  self.TogglePunct(SemiColon)
}

func (self *wordAPI) UCFirst() {
  if self.Is(Variable) {
    return
  }
  for i, v := range self.text {
    self.text = string(unicode.ToUpper(v)) + self.text[i+1:]
    break
  }
}

func (self *wordAPI) Reparent(para *paraAPI) {
  self.para = para
}
