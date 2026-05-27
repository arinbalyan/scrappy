package main
import (
  "fmt"
  "sort"
  "github.com/arinbalyan/scrappy/internal/model"
)
func main(){
  sites:=model.AllSites(); vals:=make([]string,0,len(sites));
  for _,s:= range sites { vals=append(vals,string(s)) }
  sort.Strings(vals)
  cols:=4
  rows := (len(vals)+cols-1)/cols
  fmt.Println("| Site | Site | Site | Site |")
  fmt.Println("|------|------|------|------|")
  for r:=0;r<rows;r++{
    fmt.Print("|")
    for c:=0;c<cols;c++{
      i:=r + c*rows
      if i < len(vals){ fmt.Printf(" `%s` |", vals[i]) } else { fmt.Print("  |") }
    }
    fmt.Println()
  }
}
