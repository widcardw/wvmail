export interface Reminder {
  id: string
  title: string
  /** Epoch seconds (matches the project-wide GORM timestamp convention). */
  time: number
  done: boolean
}
