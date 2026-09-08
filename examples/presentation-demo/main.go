package main
import (
 "fmt"
 "github.com/SuperMarioYL/survmap/internal/session"
 "github.com/SuperMarioYL/survmap/internal/attribution"
 sg "github.com/SuperMarioYL/survmap/internal/git"
)
func main(){
 turns:=[]session.Turn{{ID:"turn-1",Edits:[]session.FileEdit{{Path:"main.go",Tool:"Write",AddedLines:[]string{"keep()","old()"}}}},{ID:"turn-2",Edits:[]session.FileEdit{{Path:"main.go",Tool:"Edit",AddedLines:[]string{"temporary()"}}}}}
 head:=map[string][]sg.BlameLine{"main.go":{{Path:"main.go",LineNo:1,CommitSHA:"fixture-head",Content:"keep()"},{Path:"main.go",LineNo:2,CommitSHA:"fixture-head",Content:"new()"}}}
 result:=attribution.ScoreAgainstHEAD(turns,head,"fixture-head")
 for _,t:=range result.Turns{fmt.Printf("%s: %s; survived=%d churned=%d\n",t.TurnID,t.Status,t.SurvivedLines,t.ChurnedLines)}
 fmt.Printf("total: %d/%d substantive lines present\n",result.TotalSurvived,result.TotalAdded)
}
