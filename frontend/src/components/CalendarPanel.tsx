import { useState } from "react";
import { useExecutorData } from "../api/hooks";
import type { CalendarEvent, Notification, RunningTask } from "../types/api";
import "./CalendarPanel.css";

const TYPE_ICONS: Record<Notification["type"], string> = {
  task_running: "⚙",
  task_completed: "✓",
  task_failed: "✗",
  review_required: "⚠",
  calendar_conflict: "⚡",
};

const TYPE_COLORS: Record<Notification["type"], string> = {
  task_running: "var(--structural)",
  task_completed: "var(--signal)",
  task_failed: "var(--alert)",
  review_required: "var(--alert)",
  calendar_conflict: "var(--alert)",
};

const SOURCE_COLORS: Record<string, string> = {
  google: "#4285F4",
  mac: "#007AFF",
  workflow: "var(--signal)",
};

export function CalendarPanel() {
  const { calendar, notifications, tasks, loading, error, markRead } = useExecutorData();
  const [activeTab, setActiveTab] = useState<"calendar" | "notifications" | "tasks">("notifications");

  const now = new Date();
  const todayEvents = calendar.filter((e) => new Date(e.start).toDateString() === now.toDateString());
  const upcomingEvents = calendar
    .filter((e) => new Date(e.start) > now && new Date(e.start).toDateString() !== now.toDateString())
    .slice(0, 5);

  const unreadCount = notifications.filter((n) => !n.read).length;
  const runningCount = tasks.filter((t) => t.status === "running").length;

  if (loading) {
    return (
      <aside className="calendar-panel" role="complementary" aria-label="Calendar & Notifications">
        <div className="calendar-panel__loading">Loading executor data...</div>
      </aside>
    );
  }

  if (error) {
    return (
      <aside className="calendar-panel" role="complementary" aria-label="Calendar & Notifications">
        <div className="calendar-panel__error">Error: {error}</div>
      </aside>
    );
  }

  return (
    <aside className="calendar-panel" role="complementary" aria-label="Calendar & Notifications">
      <header className="calendar-panel__header">
        <h2 className="calendar-panel__title">Executor Status</h2>
        <div className="calendar-panel__tabs">
          {[
            { key: "calendar", label: "Calendar", count: todayEvents.length + upcomingEvents.length },
            { key: "notifications", label: "Notifications", count: unreadCount },
            { key: "tasks", label: "Running Tasks", count: runningCount },
          ].map((tab) => (
            <button
              key={tab.key}
              className={`calendar-panel__tab ${activeTab === tab.key ? "calendar-panel__tab--active" : ""}`}
              onClick={() => setActiveTab(tab.key as typeof activeTab)}
            >
              <span>{tab.label}</span>
              {tab.count > 0 && <span className="calendar-panel__badge">{tab.count}</span>}
            </button>
          ))}
        </div>
      </header>

      <div className="calendar-panel__content">
        {activeTab === "calendar" && (
          <CalendarView todayEvents={todayEvents} upcomingEvents={upcomingEvents} />
        )}
        {activeTab === "notifications" && <NotificationsView notifications={notifications} onMarkRead={markRead} />}
        {activeTab === "tasks" && <TasksView tasks={tasks} />}
      </div>
    </aside>
  );
}

function CalendarView({ todayEvents, upcomingEvents }: { todayEvents: CalendarEvent[]; upcomingEvents: CalendarEvent[] }) {
  return (
    <div className="calendar-view">
      {todayEvents.length > 0 && (
        <section className="calendar-view__section">
          <h3 className="calendar-view__section-title">Today</h3>
          <ul className="calendar-view__list">
            {todayEvents.map((event) => (
              <li key={event.id} className="calendar-view__item">
                <EventRow event={event} />
              </li>
            ))}
          </ul>
        </section>
      )}
      {upcomingEvents.length > 0 && (
        <section className="calendar-view__section">
          <h3 className="calendar-view__section-title">Upcoming</h3>
          <ul className="calendar-view__list">
            {upcomingEvents.map((event) => (
              <li key={event.id} className="calendar-view__item">
                <EventRow event={event} />
              </li>
            ))}
          </ul>
        </section>
      )}
      {(todayEvents.length === 0 && upcomingEvents.length === 0) && (
        <p className="calendar-view__empty">No events scheduled</p>
      )}
    </div>
  );
}

function EventRow({ event }: { event: CalendarEvent }) {
  const start = new Date(event.start);
  const end = new Date(event.end);
  const timeStr = `${start.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })}–${end.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })}`;

  return (
    <div className="calendar-view__row">
      <div className="calendar-view__time" style={{ borderLeftColor: SOURCE_COLORS[event.source] }}>
        <span className="calendar-view__time-text">{timeStr}</span>
        <span className="calendar-view__source" style={{ background: SOURCE_COLORS[event.source] }}>{event.source}</span>
      </div>
      <div className="calendar-view__details">
        <div className="calendar-view__title">{event.title}</div>
        {event.meetingUrl && (
          <a href={event.meetingUrl} target="_blank" rel="noopener" className="calendar-view__link">
            Join →
          </a>
        )}
        {event.attendees && event.attendees.length > 0 && (
          <div className="calendar-view__attendees">
            {event.attendees.slice(0, 3).map((a, i) => (
              <span key={i} className="calendar-view__attendee">{a}</span>
            ))}
            {event.attendees.length > 3 && <span className="calendar-view__attendee">+{event.attendees.length - 3} more</span>}
          </div>
        )}
      </div>
    </div>
  );
}

function NotificationsView({ notifications, onMarkRead }: { notifications: Notification[]; onMarkRead: (id: string) => void }) {
  return (
    <div className="notifications-view">
      <ul className="notifications-view__list">
        {notifications.map((notif) => (
          <li key={notif.id} className={`notifications-view__item ${!notif.read ? "notifications-view__item--unread" : ""}`}>
            <div className="notifications-view__icon" style={{ color: TYPE_COLORS[notif.type] }}>
              {TYPE_ICONS[notif.type]}
            </div>
            <div className="notifications-view__content">
              <div className="notifications-view__header">
                <span className="notifications-view__title">{notif.title}</span>
                <time className="notifications-view__time" dateTime={notif.timestamp}>
                  {new Date(notif.timestamp).toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })}
                </time>
              </div>
              <p className="notifications-view__message">{notif.message}</p>
              {notif.actionUrl && (
                <a href={notif.actionUrl} className="notifications-view__action" onClick={(e) => { e.preventDefault(); onMarkRead(notif.id); }}>
                  View →
                </a>
              )}
            </div>
          </li>
        ))}
      </ul>
      {notifications.length === 0 && <p className="notifications-view__empty">No notifications</p>}
    </div>
  );
}

function TasksView({ tasks }: { tasks: RunningTask[] }) {
  return (
    <div className="tasks-view">
      <ul className="tasks-view__list">
        {tasks.map((task) => (
          <li key={task.id} className="tasks-view__item">
            <div className="tasks-view__header">
              <span className="tasks-view__name">{task.name}</span>
              <span className={`tasks-view__status tasks-view__status--${task.status}`}>{task.status}</span>
            </div>
            <div className="tasks-view__meta">
              <span className="tasks-view__step">{task.currentStep}</span>
              <span className="tasks-view__started">Started {new Date(task.startedAt).toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })}</span>
            </div>
            {task.progress !== undefined && (
              <div className="tasks-view__progress" role="progressbar" aria-valuenow={Math.round(task.progress * 100)} aria-valuemin={0} aria-valuemax={100}>
                <div className="tasks-view__progress-bar" style={{ width: `${task.progress * 100}%` }} />
              </div>
            )}
          </li>
        ))}
      </ul>
      {tasks.length === 0 && <p className="tasks-view__empty">No running tasks</p>}
    </div>
  );
}