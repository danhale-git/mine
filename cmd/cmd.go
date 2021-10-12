package cmd

import (
	"fmt"
	"log"
	"path/filepath"
	"strconv"

	"github.com/danhale-git/mine/world"
	"github.com/spf13/cobra"
)

const worldDirPath = `C:\Users\danha\AppData\Local\Packages\Microsoft.MinecraftUWP_8wekyb3d8bbwe\LocalState\games\com.mojang\minecraftWorlds\`

//const worldFileName = `VsgSYaaGAAA=` // MINETEST  16 64 16
const worldFileName = `97caYQjdAgA=` // MINETESTFLAT 0 0 0
// TODO: Why does chunk 0 0 0 have a bkock storage version of 9?

func Init() error {
	root := &cobra.Command{
		Use: "mine <x> <y> <z>",
		//Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			w, err := world.New(filepath.Join(worldDirPath, worldFileName))
			if err != nil {
				log.Fatal(err)
			}

			b, err := w.GetBlock(
				atoi(args[0]),
				atoi(args[1]),
				atoi(args[2]),
				0,
			)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Println(b)

			/*c, err := strconv.Atoi(args[0])
			if err != nil {
				log.Fatalf("invalid argument '%s': %s", args[0], err)
			}*/

			/*i := 0
			for x := 0; x < 16; x++ {
				for z := 0; z < 16; z++ {
					for y := 0; y < 16; y++ {
						b, err := w.GetBlock(x, y, z, 0)
						if err != nil {
							if errors.Is(err, &world.SubChunkNotSavedError{}) {
								continue
							}
							log.Fatal(err)
						}
						i++
						time.Sleep(100)

						fmt.Println(b)
					}
				}
			}*/
		},
	}

	return root.Execute()
}

func atoi(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		log.Fatalf("invalid arg: '%s'", s)
	}

	return i
}
