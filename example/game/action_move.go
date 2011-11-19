package game

import "glop/util/algorithm"

func init() {
  registerActionType("move", &ActionMove{})
}
type ActionMove struct {
  basicIcon
  nonInterrupt
  Ent       *Entity

  reachable []BoardPos
  path      []BoardPos
}

func (a *ActionMove) Prep() bool {
  level := a.Ent.level
  bx := int(a.Ent.pos.X)
  by := int(a.Ent.pos.Y)
  graph := &unitGraph{level, a.Ent}
  reachable := algorithm.ReachableWithinLimit(graph, []int{level.toVertex(bx, by)}, float64(a.Ent.CurAp()))

  if len(reachable) == 0 {
    return false
  }

  vertex_to_boardpos := func(v interface{}) interface{} {
    return level.MakeBoardPosFromVertex(v.(int))
  }

  a.reachable = algorithm.Map(reachable, []BoardPos{}, vertex_to_boardpos).([]BoardPos)

  // Since this is a valid action we can go ahead and highlight all of the
  // tiles that the unit can move to
  for _,v := range a.reachable {
    level.GetCellAtPos(v).highlight |= Reachable
  }

  return true
}

func (a *ActionMove) Cancel() {
  a.reachable = nil
  a.path = nil
  a.Ent.level.clearCache(Reachable)
}

func (a *ActionMove) MouseOver(bx,by float64) {
  // TODO: Might want to highlight the specific path that would be taken if
  // the user clicked here
}

func (a *ActionMove) MouseClick(bx,by float64) ActionCommit {
  level := a.Ent.level
  dst := MakeBoardPos(int(bx), int(by))
  found := false
  for _,v := range a.reachable {
    if dst.IntEquals(v) {
      found = true
      break
    }
  }
  if !found { return NoAction }

  graph := &unitGraph{level, a.Ent}
  ap, path := algorithm.Dijkstra(graph, []int{a.Ent.pos.Vertex(a.Ent.level)}, []int{dst.Vertex(a.Ent.level)})
  if len(path) <= 1 || int(ap) > a.Ent.CurAp() {
    return NoAction
  }

  vertex_to_boardpos := func(v interface{}) interface{} {
    return level.MakeBoardPosFromVertex(v.(int))
  }

  a.path = algorithm.Map(path[1:], []BoardPos{}, vertex_to_boardpos).([]BoardPos)
  a.reachable = nil

  level.clearCache(Reachable)
  for _,v := range a.path {
    level.GetCellAtPos(v).highlight |= Reachable
  }
  if !payForMove(a.Ent, a.path[0]) {
    a.path = nil
    return NoAction
  }
  return StandardAction
}

func (a *ActionMove) Pause() bool {
  a.Ent.s.Command("stop")
  a.Cancel()
  return false
}

func (a *ActionMove) Maintain(dt int64) MaintenanceStatus {
  plen := len(a.path)
  if AdvanceEntity(a.Ent, &a.path, dt) {
    a.Cancel()
    return Complete
  }
  if len(a.path) < plen {
    return CheckForInterrupts
  }
  return InProgress
}
