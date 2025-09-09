package state

import (
    "errors"
    "fmt"
    "strings"
)

type Color int

const (
    None Color = iota
    Black
    White
)

func (c Color) Opp() Color {
    switch c {
    case Black:
        return White
    case White:
        return Black
    default:
        return None
    }
}

func (c Color) String() string {
    switch c {
    case Black:
        return "B"
    case White:
        return "W"
    default:
        return "N"
    }
}

type Game struct {
    Size int        `json:"size"`
    Turn Color      `json:"turn"`
    Board [][]Color `json:"board"`
    Captured map[Color]int `json:"captured"`

    lastMove struct{X,Y int}
    koPoint *Point

    passes int
    over bool
    result string

    scoring bool
    dead map[Point]bool
}

type Point struct{X, Y int}

func NewGame(size int) *Game {
    g := &Game{Size: size, Turn: Black, Captured: map[Color]int{Black:0, White:0}}
    g.Board = make([][]Color, size)
    for i:=0;i<size;i++{
        g.Board[i]=make([]Color, size)
    }
    return g
}

func (g *Game) inBounds(x,y int) bool {
    return x>=0 && y>=0 && x<g.Size && y<g.Size
}

func (g *Game) neighbors(x,y int) [][2]int {
    dirs := [][2]int{{1,0},{-1,0},{0,1},{0,-1}}
    res := make([][2]int,0,4)
    for _,d := range dirs {
        nx, ny := x+d[0], y+d[1]
        if g.inBounds(nx,ny){res=append(res,[2]int{nx,ny})}
    }
    return res
}

func (g *Game) Play(c Color, x, y int) error {
    if g.over {return errors.New("game over")}
    if c==None {return errors.New("spectator cannot play")}
    if c!=g.Turn {return errors.New("not your turn")}
    if !g.inBounds(x,y) {return errors.New("out of bounds")}
    if g.Board[x][y]!=None {return errors.New("occupied")}
    if g.koPoint!=nil && g.koPoint.X==x && g.koPoint.Y==y {return errors.New("ko")}

    // tentative place
    g.Board[x][y]=c

    // capture any adjacent opponent groups with no liberties
    captured := 0
    g.koPoint = nil
    for _, nb := range g.neighbors(x,y) {
        nx,ny := nb[0], nb[1]
        if g.Board[nx][ny]==c.Opp() {
            if libs, stones := g.countLiberties(nx,ny, c.Opp()); libs==0 {
                for _,s := range stones {
                    g.Board[s.X][s.Y]=None
                }
                captured += len(stones)
            }
        }
    }

    // check suicide
    if libs, _ := g.countLiberties(x,y,c); libs==0 {
        // undo and error
        // unless capture happened (snapback allowed if capture removes last liberty?) In Go suicide is illegal regardless of capture except special rules; standard Japanese/Korean forbid suicide; Chinese sometimes allow multi-stone suicide. We'll forbid suicide.
        g.Board[x][y]=None
        return errors.New("suicide")
    }

    // simple ko: if exactly one stone captured and the new stone group has exactly one liberty,
    // set koPoint to that liberty (the capture point).
    if captured==1 {
        if libs, _ := g.countLiberties(x,y,c); libs==1 {
            // find the single liberty
            for _, nb := range g.neighbors(x,y) {
                nx,ny := nb[0], nb[1]
                if g.Board[nx][ny]==None {
                    g.koPoint = &Point{X:nx, Y:ny}
                    break
                }
            }
        }
    }

    g.Captured[c] += captured

    g.lastMove = struct{X,Y int}{x,y}
    g.passes = 0
    g.Turn = g.Turn.Opp()
    return nil
}

func (g *Game) countLiberties(x,y int, c Color) (int, []Point) {
    if g.Board[x][y]!=c {return 0, nil}
    visited := make(map[Point]bool)
    queue := []Point{{x,y}}
    stones := []Point{}
    libs := map[Point]bool{}
    for len(queue)>0 {
        p := queue[len(queue)-1]
        queue = queue[:len(queue)-1]
        if visited[p] {continue}
        visited[p]=true
        stones=append(stones,p)
        for _, nb := range g.neighbors(p.X,p.Y) {
            nx,ny := nb[0], nb[1]
            if g.Board[nx][ny]==None {
                libs[Point{nx,ny}] = true
            } else if g.Board[nx][ny]==c {
                q := Point{nx,ny}
                if !visited[q] { queue = append(queue, q) }
            }
        }
    }
    return len(libs), stones
}

func (g *Game) Pass(c Color) {
    if g.over || c==None || c!=g.Turn {return}
    g.passes++
    g.Turn = g.Turn.Opp()
    if g.passes>=2 { g.scoring = true }
}

func (g *Game) Resign(c Color) {
    if g.over || c==None {return}
    g.over = true
    if c==Black { g.result = "W+R" } else { g.result = "B+R" }
}

func (g *Game) StartScoring() {
    if g.over {return}
    g.scoring = true
}

func (g *Game) FinalizeScore() {
    if g.over || !g.scoring { return }
    g.over = true
    g.result = g.Score()
}

func (g *Game) ToggleDeadAt(x,y int) {
    if !g.scoring || !g.inBounds(x,y) {return}
    p := Point{X:x,Y:y}
    if g.dead==nil { g.dead = map[Point]bool{} }
    g.dead[p] = !g.dead[p]
}

func (g *Game) Score() string {
    // Simple area (Chinese) scoring: score = stones on board + surrounded empty points; dead stones removed and counted as capture.
    // For brevity, we compute territory by flood fill on empty points and assign to color if all neighbors same color.
    black := g.Captured[Black]
    white := g.Captured[White]

    // copy board, remove dead
    board := make([][]Color, g.Size)
    for i:=0;i<g.Size;i++{board[i]=make([]Color,g.Size);copy(board[i], g.Board[i])}
    for p,dead := range g.dead { if dead {
        if board[p.X][p.Y]==Black { white++ } else if board[p.X][p.Y]==White { black++ }
        board[p.X][p.Y]=None
    }}

    // count stones
    for i:=0;i<g.Size;i++{
        for j:=0;j<g.Size;j++{
            if board[i][j]==Black { black++ }
            if board[i][j]==White { white++ }
        }
    }

    // flood fill empty regions
    visited := make(map[Point]bool)
    for i:=0;i<g.Size;i++{
        for j:=0;j<g.Size;j++{
            if board[i][j]!=None || visited[Point{i,j}] {continue}
            // BFS
            q := []Point{{i,j}}
            empties := []Point{}
            colors := map[Color]bool{}
            for len(q)>0 {
                p := q[len(q)-1]; q=q[:len(q)-1]
                if visited[p] {continue}
                visited[p]=true
                empties = append(empties, p)
                for _, nb := range [][2]int{{1,0},{-1,0},{0,1},{0,-1}} {
                    nx,ny := p.X+nb[0], p.Y+nb[1]
                    if nx<0 || ny<0 || nx>=g.Size || ny>=g.Size {continue}
                    if board[nx][ny]==None {
                        q = append(q, Point{nx,ny})
                    } else {
                        colors[board[nx][ny]] = true
                    }
                }
            }
            if len(colors)==1 {
                if colors[Black] { black += len(empties) }
                if colors[White] { white += len(empties) }
            }
        }
    }

    // no komi handling for brevity; set 6.5 komi favoring white
    komi := 6.5
    score := float64(black) - (float64(white)+komi)
    if score>0 { return "B+"+itoa(score) } else { return "W+"+itoa(-score) }
}

func itoa(f float64) string {
    // trim trailing .0 if present
    s := fmt.Sprintf("%.1f", f)
    if strings.HasSuffix(s, ".0") { return strings.TrimSuffix(s, ".0") }
    return s
}

// Snapshot struct for JSON messages

type Snapshot struct {
    Size int `json:"size"`
    Turn string `json:"turn"`
    Board [][]string `json:"board"`
    Captured map[string]int `json:"captured"`
    LastMove *Point `json:"lastMove,omitempty"`
    Over bool `json:"over"`
    Result string `json:"result"`
    Scoring bool `json:"scoring"`
    Dead []Point `json:"dead,omitempty"`
}

func (g *Game) Snapshot() Snapshot {
    s := Snapshot{
        Size: g.Size,
        Turn: g.Turn.String(),
        Board: make([][]string, g.Size),
        Captured: map[string]int{"B": g.Captured[Black], "W": g.Captured[White]},
        Over: g.over,
        Result: g.result,
        Scoring: g.scoring,
    }
    for i:=0;i<g.Size;i++{
        s.Board[i]=make([]string,g.Size)
        for j:=0;j<g.Size;j++{
            switch g.Board[i][j] {
            case Black: s.Board[i][j] = "B"
            case White: s.Board[i][j] = "W"
            default: s.Board[i][j] = "N"
            }
        }
    }
    if g.lastMove.X!=0 || g.lastMove.Y!=0 { s.LastMove = &Point{X:g.lastMove.X, Y:g.lastMove.Y} }
    if g.scoring && len(g.dead)>0 {
        for p,dead := range g.dead { if dead { s.Dead = append(s.Dead, p) } }
    }
    return s
}

