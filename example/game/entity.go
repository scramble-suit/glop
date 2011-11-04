package game

import (
  "math"
  "fmt"
  "glop/gui"
  "glop/sprite"
  "github.com/arbaal/mathgl"
  "json"
  "io/ioutil"
  "os"
  "path/filepath"
  "strings"
)

// contains the stats used to intialize a unit of this type
type UnitType struct {
  Name string

  // Name of the sprite that should be used to represent this unit
  Sprite string

  Health int

  AP int

  // basic combat stats, will be replaced with more interesting things later
  Attack  int
  Defense int

  // List of the names of the weapons this unit comes with
  Weapons []string

  // These attribute names are referenced against a master list of all
  // attributes and combined to determine the final attributes for this unit
  Attribute_names []string

  attributes Attributes
}

type UnitStats struct {
  // Contains base stats before any modifier for this unit type
  Base    *UnitType
  Health  int
  AP      int
  Weapons []Weapon
}

type CosmeticStats struct {
  // in board coordinates per ms
  Move_speed float32
}

type EntityStatsWindow struct {
  gui.EmbeddedWidget
  gui.BasicZone
  gui.NonResponder
  gui.NonFocuser

  ent     *Entity
  table   *gui.VerticalTable
  image   *gui.ImageBox
  name    *gui.TextLine
  health  *gui.TextLine
  ap      *gui.TextLine
  actions *gui.SelectBox

  // If this is false then events on this window will be immediately rejected
  // This is so we can have multiple windows, but only one can be used to
  // affect anything game related - so you can mouse-over units that aren't
  // under your control and see their stats, but not modify them, since they
  // aren't yours
  clickable bool
}

func MakeStatsWindow(clickable bool) *EntityStatsWindow {
  var esw EntityStatsWindow
  esw.EmbeddedWidget = &gui.BasicWidget{CoreWidget: &esw}
  esw.Request_dims.Dx = 350
  esw.Request_dims.Dy = 175
  esw.clickable = clickable

  top := gui.MakeHorizontalTable()

  esw.image = gui.MakeImageBox()
  top.AddChild(esw.image)

  esw.name = gui.MakeTextLine("standard", "", 275, 1, 1, 1, 1)
  esw.health = gui.MakeTextLine("standard", "", 275, 1, 1, 1, 1)
  esw.ap = gui.MakeTextLine("standard", "", 275, 1, 1, 1, 1)
  vert := gui.MakeVerticalTable()
  vert.AddChild(esw.name)
  vert.AddChild(esw.health)
  vert.AddChild(esw.ap)
  top.AddChild(vert)

  esw.table = gui.MakeVerticalTable()
  esw.table.AddChild(top)
  esw.actions = gui.MakeSelectImageBox([]string{}, []string{})
  esw.table.AddChild(esw.actions)

  return &esw
}

// Short-circuits the typical event-handling - if this window wasn't set to
// clickable then nothing will be able to get to it.
func (w *EntityStatsWindow) Respond(g *gui.Gui, e gui.EventGroup) bool {
  if w.clickable {
    return w.table.Respond(g, e)
  }
  return false
}
func (w *EntityStatsWindow) String() string {
  return "entity stats window"
}
func (w *EntityStatsWindow) update() {
  if w.ent == nil {
    return
  }
  w.health.SetText(fmt.Sprintf("Health: %d/%d", w.ent.Health, w.ent.Base.Health))
  w.ap.SetText(fmt.Sprintf("Ap: %d/%d", w.ent.AP, w.ent.Base.AP))
}
func (w *EntityStatsWindow) DoThink(int64, bool) {
  if w.ent == nil {
    return
  }
  w.update()
}
func (w *EntityStatsWindow) GetEntity() *Entity {
  return w.ent
}
func (w *EntityStatsWindow) SetEntity(e *Entity) {
  if e == w.ent {
    return
  }
  w.ent = e

  w.health.SetText("")
  w.ap.SetText("")
  w.name.SetText("")
  w.image.UnsetImage()
  w.table.RemoveChild(w.actions)
  if e != nil {
    thumb := e.s.Thumbnail()
    w.image.SetImageByTexture(thumb.Texture(), thumb.Dx(), thumb.Dy())
    w.name.SetText(e.Base.Name)
    var paths, names []string
    for i := range e.Weapons {
      paths = append(paths, filepath.Join(e.level.directory, "icons", e.Weapons[i].Icon()))
      names = append(names, e.Weapons[i].Icon())
    }
    w.actions = gui.MakeSelectImageBox(paths, names)
    w.table.AddChild(w.actions)
    w.actions.SetSelectedIndex(0)
    w.update()
  }
}
func (w *EntityStatsWindow) GetChildren() []gui.Widget {
  return []gui.Widget{w.table}
}
func (w *EntityStatsWindow) Draw(region gui.Region) {
  w.Render_region = region
  w.table.Draw(region)
}

// An Action represents something that a unit can do on its turn, other than
// move.  Actions are represented as icons in the EntityStatsWindow.
type Action interface {
  Icon() string
}

type Entity struct {
  UnitStats
  CosmeticStats

  // 0 indicates that the unit is unaffiliated
  side int

  s *sprite.Sprite

  level *Level

  // Board coordinates of this entity's current position
  pos      mathgl.Vec2
  prev_pos mathgl.Vec2

  // If the entity is currently moving then it will follow the vertices in path
  path [][2]int

  // set of vertices that this unit can see from its current location
  visible map[int]bool
}

// Returns total current attack modifier
func (e *Entity) CurrentAttackMod() int {
  x := int(e.pos.X)
  y := int(e.pos.Y)
  terrain := e.level.grid[x][y].Terrain
  if val,ok := e.UnitStats.Base.attributes.AttackMods[terrain]; ok {
    return val
  }
  return 0
}

// Returns total current defense modifier
func (e *Entity) CurrentDefenseMod() int {
  x := int(e.pos.X)
  y := int(e.pos.Y)
  terrain := e.level.grid[x][y].Terrain
  if val,ok := e.UnitStats.Base.attributes.DefenseMods[terrain]; ok {
    return e.Base.Defense + val
  }
  return e.Base.Defense
}

func bresenham(x, y, x2, y2 int) [][2]int {
  dx := x2 - x
  if dx < 0 {
    dx = -dx
  }
  dy := y2 - y
  if dy < 0 {
    dy = -dy
  }

  var ret [][2]int
  steep := dy > dx
  if steep {
    x, y = y, x
    x2, y2 = y2, x2
    dx, dy = dy, dx
    ret = make([][2]int, dy)[0:0]
  } else {
    ret = make([][2]int, dx)[0:0]
  }

  err := dx >> 1
  cy := y

  xstep := 1
  if x2 < x {
    xstep = -1
  }
  ystep := 1
  if y2 < y {
    ystep = -1
  }
  for cx := x; cx != x2; cx += xstep {
    if !steep {
      ret = append(ret, [2]int{cx, cy})
    } else {
      ret = append(ret, [2]int{cy, cx})
    }
    err -= dy
    if err < 0 {
      cy += ystep
      err += dx
    }
  }
  if !steep {
    ret = append(ret, [2]int{x2, cy})
  } else {
    ret = append(ret, [2]int{cy, x2})
  }
  return ret
}

func (e *Entity) addVisibleAlongLine(vision int, line [][2]int) {
  for _, v := range line {
    e.visible[e.level.toVertex(v[0], v[1])] = true
    concealment, ok := e.UnitStats.Base.attributes.LosMods[e.level.grid[v[0]][v[1]].Terrain]
    if concealment < 0 || !ok {
      break
    }
    vision -= concealment + 1
    if vision <= 0 {
      break
    }
  }
}

func (e *Entity) figureVisibility() {
  vision := e.UnitStats.Base.attributes.LosDistance
  ex := int(e.pos.X)
  ey := int(e.pos.Y)

  x := ex - vision
  if x < 0 {
    x = 0
  }
  y := ey - vision
  if y < 0 {
    y = 0
  }

  x2 := ex + vision
  if x2 >= len(e.level.grid) {
    x2 = len(e.level.grid) - 1
  }
  y2 := ey + vision
  if y2 >= len(e.level.grid[0]) {
    y2 = len(e.level.grid[0]) - 1
  }

  e.visible = make(map[int]bool, vision*vision)
  e.visible[e.level.toVertex(ex, ey)] = true
  for cx := x; cx <= x2; cx++ {
    e.addVisibleAlongLine(vision, bresenham(ex, ey, cx, y)[1:])
    e.addVisibleAlongLine(vision, bresenham(ex, ey, cx, y2)[1:])
  }
  for cy := y; cy <= y2; cy++ {
    e.addVisibleAlongLine(vision, bresenham(ex, ey, x, cy)[1:])
    e.addVisibleAlongLine(vision, bresenham(ex, ey, x2, cy)[1:])
  }
}

func (e *Entity) Coords() (x, y int) {
  return int(e.pos.X), int(e.pos.Y)
}

func (e *Entity) OnSetup() {
  e.Health = e.Base.Health
  e.AP = e.Base.AP
  e.prev_pos.Assign(&e.pos)
  e.figureVisibility()
}
func (e *Entity) OnRound() {
  e.AP = e.Base.AP
}

func (e *Entity) enterCell(x, y int) {
  graph := unitGraph{e.level, e.Base.attributes.MoveMods}
  src := e.level.toVertex(int(e.prev_pos.X), int(e.prev_pos.Y))
  dst := e.level.toVertex(x, y)
  e.AP -= int(graph.costToMove(src, dst))
  e.prev_pos.X = float32(x)
  e.prev_pos.Y = float32(y)
  if e.AP < 0 {
    // TODO: Log a warning
    e.AP = 0
  }
  e.figureVisibility()
}

func (e *Entity) advance(dist float32) {
  if len(e.path) == 0 {
    if e.s.CurState() != "ready" {
      e.s.Command("stop")
    }
    return
  }
  if e.s.CurState() != "walk" {
    e.s.Command("move")
  }
  fmt.Printf("")
  if e.s.CurAnim() != "walk" {
    return
  }
  if dist <= 0 {
    return
  }
  var b, t mathgl.Vec2
  b = e.pos
  t = mathgl.Vec2{float32(e.path[0][0]), float32(e.path[0][1])}
  t.Subtract(&b)
  moved := t.Length()
  if moved <= 1e-5 {
    e.enterCell(e.path[0][0], e.path[0][1])
    e.path = e.path[1:]
    e.advance(dist - moved)
    return
  }
  final_dist := dist
  if final_dist > moved {
    final_dist = moved
  }
  t.Normalize()
  t.Scale(final_dist)
  b.Add(&t)
  e.pos.Assign(&b)

  if moved > dist {
    e.turnToFace(mathgl.Vec2{float32(e.path[0][0]), float32(e.path[0][1])})
  }

  e.advance(dist - final_dist)
}

func (e *Entity) turnToFace(target mathgl.Vec2) {
  target.Subtract(&e.pos)
  facing := math.Atan2(float64(target.Y), float64(target.X)) / (2 * math.Pi) * 360.0
  var face int
  if facing >= 22.5 || facing < -157.5 {
    face = 0
  } else {
    face = 1
  }
  if face != e.s.StateFacing() {
    e.s.Command("turn_left")
  }
}

func (e *Entity) Think(dt int64) {
  e.s.Think(dt)
  e.advance(e.Move_speed * float32(dt))
}

func LoadAllUnits(dir string) ([]*UnitType, error) {
  var paths []string
  unit_dir := filepath.Join(dir, "units")
  err := filepath.Walk(unit_dir, func(path string, info *os.FileInfo, err error) error {
    if !info.IsDirectory() && strings.HasSuffix(path, ".json") {
      paths = append(paths, path)
    }
    return nil
  })
  if err != nil {
    panic(fmt.Sprintf("Error reading directory %s: %s\n", dir, err.Error()))
  }
  atts, err := loadAttributes(dir)
  if err != nil {
    panic(fmt.Sprintf("Error loading attributes: %s\n", err.Error()))
  }

  var units []*UnitType
  for _, path := range paths {
    f, err := os.Open(path)
    if err != nil {
      return nil, err
    }
    defer f.Close()
    data, err := ioutil.ReadAll(f)
    if err != nil {
      return nil, err
    }
    var unit UnitType
    err = json.Unmarshal(data, &unit)
    if err != nil {
      return nil, err
    }
    unit.processAttributes(atts)
    units = append(units, &unit)
  }
  return units, nil
}
