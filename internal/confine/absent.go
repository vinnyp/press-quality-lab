package confine

import "github.com/vinnyp/press-quality-lab/internal/throwaway"

func absentPlanted(parent string) (*throwaway.Workspace, string, error) {
	return throwaway.AbsentPlanted(parent)
}
