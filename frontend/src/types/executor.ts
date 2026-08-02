export type CalendarEvent = {
  id: string;
  title: string;
  start: string; // ISO 8601
  end: string;
  source: "google" | "mac" | "workflow";
  calendarId: string;
  meetingUrl?: string;
  attendees?: string[];
};

export type Notification = {
  id: string;
  type: "task_running" | "task_completed" | "task_failed" | "review_required" | "calendar_conflict";
  title: string;
  message: string;
  timestamp: string;
  read: boolean;
  relatedEvidenceId?: string;
  relatedToolCall?: string;
  actionUrl?: string;
};

export type RunningTask = {
  id: string;
  name: string;
  startedAt: string;
  status: "running" | "pending" | "awaiting_review";
  progress?: number; // 0-1
  currentStep?: string;
  evidenceId?: string;
};