package main

import (
	"fmt"
	"os"
	"raylib"

	"github.com/goplus/llgo/c"
)

func main() {
	const screenWidth = 800
	const screenHeight = 450
	blue := raylib.GetColor(0xFF00F179)
	white := raylib.GetColor(0xFFFFFFFF)

	// run in headless mode(CI) without proper graphics hardware or drivers,
	// causing OpenGL initialization to fail.
	// so we check these function which is not related to graphics initialization.
	if os.Getenv("CI") == "true" {
		raylib.SetRandomSeed(42)
		randomValue := raylib.GetRandomValue(1, 100)
		fmt.Println("Random value:", randomValue)
		color := raylib.GetColor(0xFF00F179)
		fmt.Println("Color components:", color.R, color.G, color.B, color.A)
		return
	}

	raylib.InitWindow(screenWidth, screenHeight, c.Str("Raylib DEMO"))
	startTime := raylib.GetTime()
	for !raylib.WindowShouldClose() {
		currentTime := raylib.GetTime()
		if currentTime-startTime >= 1.0 {
			break
		}
		raylib.BeginDrawing()
		raylib.ClearBackground(white)
		raylib.DrawRectangle(screenWidth/2-50, screenHeight/2-50, 100, 100, blue)
		raylib.EndDrawing()
	}
}
