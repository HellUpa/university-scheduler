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
	ID      uint `gorm:"primaryKey"`
	Name    string
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
	Duration         int      // Количество слотов
}
