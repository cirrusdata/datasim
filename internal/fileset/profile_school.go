package fileset

// schoolProfile returns the built-in school fileset profile.
func schoolProfile() Profile {
	return Profile{
		Name:        "school",
		Description: "District or campus share with departments, classrooms, media labs, and student submissions.",
		Directories: []string{
			"Administration/Board", "Classrooms/Grade-05", "Classrooms/Grade-10", "Faculty/Science", "Faculty/Arts",
			"Media/Announcements", "Students/Assignments", "Students/Projects", "Athletics/Photos", "Library/Archives",
		},
		Nouns:    []string{"lesson", "syllabus", "attendance", "project", "sciencefair", "semester", "curriculum", "student"},
		Prefixes: []string{"draft", "grading", "semester", "lecture", "club", "trip"},
		Extensions: map[string][]string{
			"archive": {".zip"},
			"code":    {".py", ".ipynb", ".json"},
			"doc":     {".docx", ".pdf", ".txt"},
			"img":     {".jpg", ".png"},
			"log":     {".log"},
			"sheet":   {".xlsx", ".csv"},
			"vid":     {".mp4"},
		},
	}
}
