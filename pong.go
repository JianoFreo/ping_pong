package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"
)

const (
	screenWidth  = 800
	screenHeight = 600

	paddleWidth  = 12
	paddleHeight = 90
	paddleSpeed  = 6.0

	ballSize   = 12
	ballSpeed  = 5.0
	aiMaxSpeed = 4.0
)

type Paddle struct {
	X, Y float64
}

type Ball struct {
	X, Y   float64
	VX, VY float64
}

type Game struct {
	player      Paddle
	ai          Paddle
	ball        Ball
	playerScore int
	aiScore     int
	randSrc     *rand.Rand
	paused      bool
	bgColor     color.RGBA
}

func NewGame() *Game {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	g := &Game{
		player:  Paddle{X: 20, Y: screenHeight/2 - paddleHeight/2},
		ai:      Paddle{X: screenWidth - 20 - paddleWidth, Y: screenHeight/2 - paddleHeight/2},
		randSrc: r,
	}
	// initialize background color and ball
	g.randomizeBackground()
	g.resetBall(true)
	return g
}

func (g *Game) randomizeBackground() {
	// Pick a random color while avoiding extremely bright backgrounds
	rr := uint8(10 + g.randSrc.Intn(230))
	gg := uint8(10 + g.randSrc.Intn(230))
	bb := uint8(10 + g.randSrc.Intn(230))
	// adjust if too bright for readable UI
	lum := 0.299*float64(rr) + 0.587*float64(gg) + 0.114*float64(bb)
	if lum > 180 {
		factor := 180.0 / lum
		rr = uint8(float64(rr) * factor)
		gg = uint8(float64(gg) * factor)
		bb = uint8(float64(bb) * factor)
	}
	g.bgColor = color.RGBA{rr, gg, bb, 255}
}

func (g *Game) resetBall(toPlayer bool) {
	g.ball.X = screenWidth/2 - ballSize/2
	g.ball.Y = screenHeight/2 - ballSize/2

	angle := (g.randSrc.Float64()*math.Pi/3 - math.Pi/6) // -30 to +30 degrees
	speed := ballSpeed
	if !toPlayer {
		angle += math.Pi // invert direction towards AI
	}
	g.ball.VX = speed * math.Cos(angle)
	g.ball.VY = speed * math.Sin(angle)
	// small random tweak so not exactly horizontal sometimes
	if math.Abs(g.ball.VY) < 0.5 {
		g.ball.VY = g.ball.VY + 0.5*(g.randSrc.Float64()-0.5)
	}
}

func (g *Game) Update() error {
	// Pause/unpause
	if ebiten.IsKeyPressed(ebiten.KeySpace) {
		// Toggle pause on first frame of press - simple debounce
		if !g.paused {
			g.paused = true
			return nil
		}
	} else {
		// release pause toggle by making sure space is not pressed
		if g.paused {
			// allow to resume by pressing space again: we consider space acts as toggle but with hold prevention
		}
	}
	// Use P to toggle pause properly
	if ebiten.IsKeyPressed(ebiten.KeyP) {
		g.paused = !g.paused
		// tiny sleep to avoid super-fast toggles (safe in small single-threaded loop)
		time.Sleep(120 * time.Millisecond)
	}
	if g.paused {
		return nil
	}

	// Player input (W/S or Up/Down)
	if ebiten.IsKeyPressed(ebiten.KeyW) || ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		g.player.Y -= paddleSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) || ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		g.player.Y += paddleSpeed
	}
	// Clamp player
	if g.player.Y < 0 {
		g.player.Y = 0
	}
	if g.player.Y > screenHeight-paddleHeight {
		g.player.Y = screenHeight - paddleHeight
	}

	// AI simple movement: follow ball with a max speed
	target := g.ball.Y + ballSize/2 - paddleHeight/2
	diff := target - g.ai.Y
	if math.Abs(diff) > 1 {
		move := math.Min(aiMaxSpeed, math.Abs(diff))
		if diff > 0 {
			g.ai.Y += move
		} else {
			g.ai.Y -= move
		}
	}
	// Clamp AI
	if g.ai.Y < 0 {
		g.ai.Y = 0
	}
	if g.ai.Y > screenHeight-paddleHeight {
		g.ai.Y = screenHeight - paddleHeight
	}

	// Move ball
	g.ball.X += g.ball.VX
	g.ball.Y += g.ball.VY

	// Collide with top/bottom
	if g.ball.Y <= 0 {
		g.ball.Y = 0
		g.ball.VY = -g.ball.VY
	}
	if g.ball.Y+ballSize >= screenHeight {
		g.ball.Y = screenHeight - ballSize
		g.ball.VY = -g.ball.VY
	}

	// Paddle collisions
	// Player paddle
	if rectsCollide(g.ball.X, g.ball.Y, ballSize, ballSize, g.player.X, g.player.Y, paddleWidth, paddleHeight) {
		g.ball.X = g.player.X + paddleWidth
		g.ball.VX = math.Abs(g.ball.VX) // go right
		// add vertical velocity based on where the ball hit the paddle
		offset := (g.ball.Y + ballSize/2) - (g.player.Y + paddleHeight/2)
		g.ball.VY = offset * 0.12
		g.randomizeBackground()
	}
	// AI paddle
	if rectsCollide(g.ball.X, g.ball.Y, ballSize, ballSize, g.ai.X, g.ai.Y, paddleWidth, paddleHeight) {
		g.ball.X = g.ai.X - ballSize
		g.ball.VX = -math.Abs(g.ball.VX) // go left
		offset := (g.ball.Y + ballSize/2) - (g.ai.Y + paddleHeight/2)
		g.ball.VY = offset * 0.12
		g.randomizeBackground()
	}

	// Scoring: left out -> AI scores, right out -> player scores
	if g.ball.X+ballSize < 0 {
		g.aiScore++
		g.resetBall(true) // ball goes towards player next
	}
	if g.ball.X > screenWidth {
		g.playerScore++
		g.resetBall(false)
	}

	// Reset scores if somebody wins (first to 7)
	if g.playerScore >= 7 || g.aiScore >= 7 {
		g.playerScore = 0
		g.aiScore = 0
		// center ball
		g.resetBall(true)
	}

	return nil
}

func rectsCollide(x1, y1, w1, h1, x2, y2, w2, h2 float64) bool {
	return x1 < x2+w2 && x1+w1 > x2 && y1 < y2+h2 && y1+h1 > y2
}

func (g *Game) Draw(screen *ebiten.Image) {
	// Background
	ebitenutil.DrawRect(screen, 0, 0, screenWidth, screenHeight, g.bgColor)

	// Center dashed line
	for y := 0; y < screenHeight; y += 20 {
		ebitenutil.DrawRect(screen, screenWidth/2-2, float64(y), 4, 12, color.RGBA{200, 200, 200, 50})
	}

	// Paddles
	ebitenutil.DrawRect(screen, g.player.X, g.player.Y, paddleWidth, paddleHeight, color.White)
	ebitenutil.DrawRect(screen, g.ai.X, g.ai.Y, paddleWidth, paddleHeight, color.White)

	// Ball
	ebitenutil.DrawRect(screen, g.ball.X, g.ball.Y, ballSize, ballSize, color.RGBA{255, 160, 0, 255})

	// Scores
	scoreText := fmt.Sprintf("%d    %d", g.playerScore, g.aiScore)
	text.Draw(screen, scoreText, basicfont.Face7x13, screenWidth/2-20, 30, color.White)

	// Controls hint
	text.Draw(screen, "W/S or ↑/↓ — move | P — pause | Space — (no-op) ", basicfont.Face7x13, 10, screenHeight-10, color.RGBA{200, 200, 200, 200})
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Pong in Go — single-player")

	game := NewGame()
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
