package domain

// Enums
type RoomType string

const (
	RoomTypeLecture  RoomType = "lecture"
	RoomTypePractice RoomType = "practice"
	RoomTypeLab      RoomType = "lab"
)

type DayOfWeek string

const (
	DayMonday    DayOfWeek = "monday"
	DayTuesday   DayOfWeek = "tuesday"
	DayWednesday DayOfWeek = "wednesday"
	DayThursday  DayOfWeek = "thursday"
	DayFriday    DayOfWeek = "friday"
	DaySaturday  DayOfWeek = "saturday"
)

// --- Models ---

type TimeSlot struct {
	ID           uint      `gorm:"primaryKey"`
	Day          DayOfWeek `gorm:"type:varchar(20)"`
	PeriodNumber int       // Номер пары (1, 2, 3...)
	StartTime    string    // "09:00" (храним как строку или time.Time)
	EndTime      string    // "10:30"
}

type Room struct {
	ID       uint   `gorm:"primaryKey"`
	Name     string `gorm:"uniqueIndex"`
	Capacity int
	Type     RoomType `gorm:"type:varchar(20)"`
	Floor    int
}

type Instructor struct {
	ID   uint `gorm:"primaryKey"`
	Name string
	// HasMany
	Classes []CourseClass `gorm:"foreignKey:InstructorID"`
}

type Group struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"uniqueIndex"`
	Size int
	// Many2Many
	Classes []*CourseClass `gorm:"many2many:group_classes;"`
}

type Subject struct {
	ID   uint `gorm:"primaryKey"`
	Name string

	// Система кредитов (учебный план)
	Credits       int // Количество зачетных единиц (например, 5)
	LectureHours  int // Сколько часов лекций в неделю (например, 2)
	PracticeHours int // Сколько часов практики в неделю (например, 4)
	LabHours      int // Сколько часов лабораторных (например, 0)

	Classes []CourseClass `gorm:"foreignKey:SubjectID"`
}

// CourseClass - занятие, которое нужно поставить в расписание
type CourseClass struct {
	ID           uint `gorm:"primaryKey"`
	SubjectID    uint
	InstructorID uint

	Subject    Subject    `gorm:"foreignKey:SubjectID"`
	Instructor Instructor `gorm:"foreignKey:InstructorID"`
	Groups     []*Group   `gorm:"many2many:group_classes;"` // M2M связь

	IsLecture        bool
	RequiredRoomType RoomType `gorm:"type:varchar(20)"`
	Duration         int      // Количество временных слотов
}
