import { Events } from "@wailsio/runtime";
import { ReminderService } from "../bindings/alarme";
import type { Reminder } from "../bindings/alarme";

const appElement = document.querySelector<HTMLElement>(".app")!;
const listElement = document.getElementById("reminder-list")!;
const popupAlertElement = document.getElementById("popup-alert")!;
const popupRemindersElement = document.getElementById("popup-reminders")!;
const popupTimeElement = document.getElementById("popup-time")! as HTMLTimeElement;
const dialog = document.getElementById("reminder-dialog")! as HTMLDialogElement;
const dialogTitle = document.getElementById("dialog-title")!;
const form = document.getElementById("reminder-form")! as HTMLFormElement;
const nameInput = document.getElementById("reminder-name")! as HTMLInputElement;
const soundButtons = Array.from(document.querySelectorAll<HTMLButtonElement>(".sound-option"));
const soundSelect = document.getElementById("sound-select")!;
const soundSelectTrigger = document.getElementById("sound-select-trigger")! as HTMLButtonElement;
const soundSelectMenu = document.getElementById("sound-select-list")!;
const soundSelectedIcon = document.getElementById("sound-selected-icon")!;
const soundSelectedName = document.getElementById("sound-selected-name")!;
const dateInput = document.getElementById("reminder-date")! as HTMLInputElement;
const timeInput = document.getElementById("reminder-time")! as HTMLInputElement;
const hourInput = document.getElementById("time-hour")! as HTMLInputElement;
const minuteInput = document.getElementById("time-minute")! as HTMLInputElement;
const datePickerWrap = document.getElementById("date-picker-wrap")!;
const dateField = document.getElementById("date-field")! as HTMLFieldSetElement;
const dateTrigger = document.getElementById("date-trigger")! as HTMLButtonElement;
const dateTriggerLabel = document.getElementById("date-trigger-label")!;
const calendarMonthLabel = document.getElementById("calendar-month")!;
const calendarDays = document.getElementById("calendar-days")!;
const selectedDateLabel = document.getElementById("selected-date-label")!;
const weekdaysField = document.getElementById("weekdays-field")!;
const oneTimeInput = document.getElementById("one-time-event")! as HTMLInputElement;
const formError = document.getElementById("form-error")!;
const pageError = document.getElementById("error")!;
const saveButton = document.getElementById("save-reminder")! as HTMLButtonElement;
const deleteButton = document.getElementById("delete-reminder")! as HTMLButtonElement;
const themeToggle = document.getElementById("theme-toggle")! as HTMLInputElement;
const settingsDialog = document.getElementById("settings-dialog")! as HTMLDialogElement;
const startupStatus = document.getElementById("startup-status")!;
const startupAddButton = document.getElementById("startup-add")! as HTMLButtonElement;
const startupWarning = document.getElementById("startup-warning")!;
const startupWarningMessage = document.getElementById("startup-warning-message")!;
const startupWarningAddButton = document.getElementById("startup-warning-add")! as HTMLButtonElement;

const weekdayNames = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
const selectedWeekdays = new Set<number>();
const previousTimeValues = new Map<HTMLInputElement, string>();
const isPopupWindow = new URLSearchParams(window.location.search).has("popup");
const soundSources: Record<string, string> = {
  "soft-chime": "/sounds/soft-chime.wav",
  "glass-bells": "/sounds/glass-bells.wav",
  "warm-tone": "/sounds/warm-tone.wav",
  dewdrop: "/sounds/dewdrop.wav",
  aurora: "/sounds/aurora.wav",
  pulse: "/sounds/pulse.wav",
  twinkle: "/sounds/twinkle.wav",
  "moon-sun": "/sounds/moon-sun.wav",
  "rain-glass": "/sounds/rain-glass.wav",
  hearth: "/sounds/hearth.wav",
  orbit: "/sounds/orbit.wav",
  "ode-to-joy": "/sounds/ode-to-joy.wav",
  "hungarian-march": "/sounds/hungarian-march.wav",
  "happy-birthday": "/sounds/happy-birthday.wav",
};
const notificationQueue: string[] = [];
let selectedSound = "none";
let previewAudio: HTMLAudioElement | undefined;
let notificationAudio: HTMLAudioElement | undefined;
let closeTimer: ReturnType<typeof setTimeout> | undefined;
let settingsCloseTimer: ReturnType<typeof setTimeout> | undefined;
let editingReminderId: string | undefined;
let displayedMonth = new Date(new Date().getFullYear(), new Date().getMonth(), 1);

function selectSound(sound: string) {
  selectedSound = soundSources[sound] || sound === "none" ? sound : "none";
  const selectedButton = soundButtons.find((button) => button.dataset.soundId === selectedSound)!;
  soundButtons.forEach((button) => {
    const isSelected = button.dataset.soundId === selectedSound;
    button.classList.toggle("is-selected", isSelected);
    button.setAttribute("aria-selected", String(isSelected));
  });
  const optionIcon = selectedButton.querySelector(".sound-option-icon")!;
  soundSelectedIcon.replaceChildren(...Array.from(optionIcon.childNodes, (node) => node.cloneNode(true)));
  soundSelectedName.textContent = selectedButton.querySelector(".sound-option-copy strong")!.textContent;
  soundSelectTrigger.setAttribute("aria-label", `Notification sound: ${soundSelectedName.textContent}`);
}

function closeSoundMenu(returnFocus = false) {
  if (soundSelectMenu.matches(":popover-open")) {
    soundSelectMenu.hidePopover();
  }
  soundSelectMenu.hidden = true;
  soundSelectTrigger.setAttribute("aria-expanded", "false");
  if (returnFocus) {
    soundSelectTrigger.focus();
  }
}

function openSoundMenu() {
  soundSelectMenu.hidden = false;
  soundSelectMenu.showPopover();
  soundSelectTrigger.setAttribute("aria-expanded", "true");
  positionSoundMenu();
  soundButtons.find((button) => button.dataset.soundId === selectedSound)?.focus();
}

function positionSoundMenu() {
  const triggerRect = soundSelectTrigger.getBoundingClientRect();
  const inset = 12;
  const gap = 6;
  const width = Math.min(triggerRect.width, window.innerWidth - inset * 2);
  const left = Math.min(Math.max(triggerRect.left, inset), window.innerWidth - inset - width);
  soundSelectMenu.style.width = `${width}px`;
  soundSelectMenu.style.left = `${left}px`;
  soundSelectMenu.style.top = `${inset}px`;
  const height = soundSelectMenu.getBoundingClientRect().height;
  const availableAbove = triggerRect.top - inset - gap;
  const availableBelow = window.innerHeight - inset - triggerRect.bottom - gap;
  const placeAbove = height > availableBelow && availableAbove > availableBelow;
  const top = placeAbove
    ? Math.max(inset, triggerRect.top - gap - height)
    : Math.min(triggerRect.bottom + gap, window.innerHeight - inset - height);
  soundSelectMenu.style.top = `${top}px`;
}

function previewSound(sound: string) {
  previewAudio?.pause();
  const source = soundSources[sound];
  if (!source) {
    previewAudio = undefined;
    return;
  }
  previewAudio = new Audio(source);
  previewAudio.volume = 0.72;
  void previewAudio.play().catch((error) => console.warn("Could not preview reminder sound:", error));
}

function playNextNotificationSound() {
  if (notificationAudio || notificationQueue.length === 0) {
    return;
  }
  const sound = notificationQueue.shift()!;
  const source = soundSources[sound];
  if (!source) {
    playNextNotificationSound();
    return;
  }
  const audio = new Audio(source);
  notificationAudio = audio;
  audio.volume = 0.82;
  const finish = () => {
    if (notificationAudio !== audio) {
      return;
    }
    notificationAudio = undefined;
    playNextNotificationSound();
  };
  audio.addEventListener("ended", finish, { once: true });
  audio.addEventListener("error", finish, { once: true });
  void audio.play().catch((error) => {
    console.warn("Could not play reminder sound:", error);
    finish();
  });
}

function queueNotificationSound(sound: string) {
  if (soundSources[sound]) {
    notificationQueue.push(sound);
    playNextNotificationSound();
  }
}

function localDateString(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function parseLocalDate(value: string): Date | undefined {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) {
    return undefined;
  }
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
  return localDateString(date) === value ? date : undefined;
}

function renderCalendar() {
  const year = displayedMonth.getFullYear();
  const month = displayedMonth.getMonth();
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const todayValue = localDateString(today);
  const selectedDate = parseLocalDate(dateInput.value);
  const firstDayOffset = (new Date(year, month, 1).getDay() + 6) % 7;
  const daysInMonth = new Date(year, month + 1, 0).getDate();
  const cellCount = Math.ceil((firstDayOffset + daysInMonth) / 7) * 7;

  calendarMonthLabel.textContent = new Intl.DateTimeFormat("en-US", {
    month: "long",
    year: "numeric",
  }).format(displayedMonth);
  calendarDays.replaceChildren();

  for (let cell = 0; cell < cellCount; cell++) {
    const day = cell - firstDayOffset + 1;
    if (day < 1 || day > daysInMonth) {
      const empty = document.createElement("span");
      empty.className = "calendar-empty";
      empty.setAttribute("aria-hidden", "true");
      calendarDays.append(empty);
      continue;
    }

    const date = new Date(year, month, day);
    const value = localDateString(date);
    const button = document.createElement("button");
    button.type = "button";
    button.className = "calendar-day";
    button.textContent = String(day);
    button.setAttribute("aria-label", new Intl.DateTimeFormat("en-US", {
      weekday: "long",
      month: "long",
      day: "numeric",
      year: "numeric",
    }).format(date));
    button.setAttribute("aria-pressed", String(value === dateInput.value));
    button.disabled = value < todayValue;
    if (value === todayValue) {
      button.classList.add("is-today");
    }
    if (value === dateInput.value) {
      button.classList.add("is-selected");
    }
    button.addEventListener("click", () => {
      dateInput.value = value;
      renderCalendar();
    });
    calendarDays.append(button);
  }

  selectedDateLabel.textContent = selectedDate
    ? new Intl.DateTimeFormat("en-US", {
        weekday: "short",
        month: "short",
        day: "numeric",
        year: "numeric",
      }).format(selectedDate)
    : "Select a date";
  dateTriggerLabel.textContent = selectedDate
    ? new Intl.DateTimeFormat("en-US", {
        month: "long",
        day: "numeric",
        year: "numeric",
      }).format(selectedDate)
    : "Choose a date";
  const currentMonth = new Date(today.getFullYear(), today.getMonth(), 1);
  document.getElementById("previous-month")!.toggleAttribute(
    "disabled",
    displayedMonth.getTime() <= currentMonth.getTime(),
  );
}

function showCalendarForDate(value: string) {
  const date = parseLocalDate(value) ?? new Date();
  displayedMonth = new Date(date.getFullYear(), date.getMonth(), 1);
  renderCalendar();
}

function setTimeControls(time: string) {
  const [hours = "09", minutes = "00"] = time.split(":");
  hourInput.value = hours.padStart(2, "0");
  minuteInput.value = minutes.padStart(2, "0");
  previousTimeValues.set(hourInput, hourInput.value);
  previousTimeValues.set(minuteInput, minuteInput.value);
  syncTimeValue();
}

function animateTimeChange(input: HTMLInputElement, direction: number) {
  input.classList.remove("time-change-up", "time-change-down");
  void input.offsetWidth;
  input.classList.add(direction >= 0 ? "time-change-up" : "time-change-down");
}

function syncTimeValue() {
  const hours = Number(hourInput.value);
  const minutes = Number(minuteInput.value);
  if (
    hourInput.value.length === 2 &&
    minuteInput.value.length === 2 &&
    Number.isInteger(hours) &&
    Number.isInteger(minutes) &&
    hours >= 0 &&
    hours <= 23 &&
    minutes >= 0 &&
    minutes <= 59
  ) {
    timeInput.value = `${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}`;
  }
}

function stepTime(unit: "hour" | "minute", amount: number) {
  const input = unit === "hour" ? hourInput : minuteInput;
  const maximum = unit === "hour" ? 23 : 59;
  const current = Number(input.value);
  const value = Number.isInteger(current) && current >= 0 && current <= maximum ? current : 0;
  const next = String((value + amount + maximum + 1) % (maximum + 1)).padStart(2, "0");
  if (input.value !== next) {
    animateTimeChange(input, amount);
    input.value = next;
    previousTimeValues.set(input, next);
  }
  syncTimeValue();
}

function normalizeTimeInput(input: HTMLInputElement, maximum: number) {
  const parsed = Number(input.value);
  const value = input.value === "" || !Number.isInteger(parsed) ? 0 : Math.min(Math.max(parsed, 0), maximum);
  input.value = String(value).padStart(2, "0");
  syncTimeValue();
}

function showPageError(message: string) {
  pageError.textContent = message;
  pageError.hidden = false;
}

function formatError(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);
  return `Error: ${message.replace(/^RuntimeError:\s*/, "")}`;
}

function updateThemeToggle() {
  const isLight = document.documentElement.dataset.theme === "light";
  themeToggle.checked = !isLight;
  document.querySelector('meta[name="theme-color"]')!.setAttribute("content", isLight ? "#f4f5f7" : "#0f1118");
}

async function refreshStartupSetting() {
  startupAddButton.disabled = true;
  startupStatus.textContent = "Checking…";
  try {
    const enabled = await ReminderService.StartupEnabled();
    startupStatus.textContent = enabled ? "Already enabled" : "Not enabled";
    startupAddButton.textContent = enabled ? "Added" : "Add";
    startupAddButton.disabled = enabled;
  } catch (error) {
    startupStatus.textContent = formatError(error);
    startupAddButton.textContent = "Add";
  }
}

async function refreshStartupWarning(hasReminders: boolean) {
  startupWarning.hidden = true;
  if (!hasReminders) {
    return;
  }
  try {
    if (await ReminderService.StartupEnabled()) {
      return;
    }
    startupWarningMessage.textContent = "Alarme is not set to start with Windows.";
    startupWarning.hidden = false;
  } catch (error) {
    startupWarningMessage.textContent = `Could not check Windows startup: ${formatError(error)}`;
    startupWarning.hidden = false;
  }
}

function setTheme(theme: "dark" | "light") {
  document.documentElement.dataset.theme = theme;
  try {
    localStorage.setItem("alarme-theme", theme);
  } catch (error) {
    console.error("Could not save theme preference:", error);
  }
  updateThemeToggle();
  void Events.Emit("theme:changed", theme).catch((error) => {
    console.error("Could not synchronize reminder window theme:", error);
  });
}

function clearErrors() {
  pageError.hidden = true;
  formError.hidden = true;
}

function openDialog(reminder?: Reminder) {
  clearErrors();
  form.reset();
  closeSoundMenu();
  selectSound(reminder?.sound ?? "none");
  editingReminderId = reminder?.id;
  dialogTitle.textContent = reminder ? "Edit alarm" : "Add alarm";
  if (reminder) {
    nameInput.value = reminder.name;
    setTimeControls(reminder.time);
    dateInput.value = reminder.date ?? localDateString(new Date());
    oneTimeInput.checked = reminder.type === "weekly";
    saveButton.textContent = "Save";
    deleteButton.hidden = false;
  } else {
    const defaultDateTime = new Date();
    defaultDateTime.setMinutes(defaultDateTime.getMinutes() + 1);
    dateInput.value = localDateString(defaultDateTime);
    const defaultTime = `${String(defaultDateTime.getHours()).padStart(2, "0")}:${String(defaultDateTime.getMinutes()).padStart(2, "0")}`;
    setTimeControls(defaultTime);
    oneTimeInput.checked = false;
    saveButton.textContent = "Save";
    deleteButton.hidden = true;
  }
  showCalendarForDate(dateInput.value);
  selectedWeekdays.clear();
  if (reminder?.type === "weekly") {
    reminder.weekdays?.forEach((day) => selectedWeekdays.add(day));
  } else {
    selectedWeekdays.add(new Date().getDay());
  }
  document.querySelectorAll<HTMLButtonElement>(".weekday").forEach((button) => {
    button.setAttribute("aria-pressed", String(selectedWeekdays.has(Number(button.dataset.day))));
  });
  updateScheduleFields();
  dialog.showModal();
  requestAnimationFrame(() => dialog.classList.add("is-open"));
  window.setTimeout(() => nameInput.focus(), 50);
}

function closeDialog() {
  closeCalendar();
  closeSoundMenu();
  dialog.classList.remove("is-open");
  if (closeTimer) {
    clearTimeout(closeTimer);
  }
  closeTimer = window.setTimeout(() => {
    if (dialog.open) {
      dialog.close();
    }
  }, 180);
}

function closeSettingsDialog() {
  settingsDialog.classList.remove("is-open");
  if (settingsCloseTimer) {
    clearTimeout(settingsCloseTimer);
  }
  settingsCloseTimer = window.setTimeout(() => {
    if (settingsDialog.open) {
      settingsDialog.close();
    }
  }, 180);
}

function updateScheduleFields() {
  const isDate = !oneTimeInput.checked;
  datePickerWrap.hidden = !isDate;
  weekdaysField.hidden = isDate;
  dateInput.required = isDate;
  if (!isDate) {
    closeCalendar();
  }
}

function openCalendar() {
  showCalendarForDate(dateInput.value);
  dateField.hidden = false;
  dateField.showPopover();
  positionCalendar();
  dateTrigger.setAttribute("aria-expanded", "true");
}

function closeCalendar() {
  if (dateField.matches(":popover-open")) {
    dateField.hidePopover();
  }
  dateField.hidden = true;
  dateTrigger.setAttribute("aria-expanded", "false");
}

function positionCalendar() {
  const dialogRect = dialog.getBoundingClientRect();
  const triggerRect = dateTrigger.getBoundingClientRect();
  const inset = 12;
  const gap = 8;
  const width = Math.min(triggerRect.width, dialogRect.width - inset * 2, window.innerWidth - inset * 2);
  const left = Math.min(
    Math.max(triggerRect.left, dialogRect.left + inset, inset),
    window.innerWidth - inset - width,
  );

  dateField.style.width = `${width}px`;
  dateField.style.left = `${left}px`;
  dateField.style.top = `${inset}px`;
  const height = dateField.getBoundingClientRect().height;
  const availableAbove = triggerRect.top - inset - gap;
  const availableBelow = window.innerHeight - inset - triggerRect.bottom - gap;
  const placeAbove = height <= availableAbove || availableAbove >= availableBelow;
  const top = placeAbove
    ? Math.max(inset, triggerRect.top - gap - height)
    : Math.min(triggerRect.bottom + gap, window.innerHeight - inset - height);
  dateField.style.top = `${top}px`;
}

function formatReminderSchedule(reminder: Reminder): string {
  if (reminder.type === "date") {
    const dateString = reminder.date ?? "";
    const today = new Date();
    const todayString = localDateString(today);
    const tomorrowString = localDateString(new Date(today.getFullYear(), today.getMonth(), today.getDate() + 1));
    if (dateString === todayString) {
      return "Today";
    }
    if (dateString === tomorrowString) {
      return "Tomorrow";
    }

    const date = new Date(`${dateString}T00:00:00`);
    const options: Intl.DateTimeFormatOptions = { day: "numeric", month: "long" };
    if (date.getFullYear() !== today.getFullYear()) {
      options.year = "numeric";
    }
    return new Intl.DateTimeFormat("en-US", options).format(date);
  }
  const weekdays = reminder.weekdays ?? [];
  if (new Set(weekdays).size === weekdayNames.length) {
    return "Everyday";
  }
  return weekdays.map((day) => weekdayNames[day].slice(0, 3)).join(", ");
}

function showDueReminder(reminder: Reminder): boolean {
  if (popupRemindersElement.querySelector(`[data-reminder-id="${CSS.escape(reminder.id)}"]`)) {
    return false;
  }

  if (!popupRemindersElement.childElementCount) {
    popupTimeElement.textContent = reminder.time;
    popupTimeElement.dateTime = reminder.time;
  }

  const alert = document.createElement("article");
  alert.className = "popup-reminder";
  alert.dataset.reminderId = reminder.id;

  const title = document.createElement("h2");
  title.textContent = reminder.name;

  alert.append(title);
  popupRemindersElement.append(alert);
  popupAlertElement.hidden = false;
  return true;
}

async function loadReminders() {
  try {
    const reminders = (await ReminderService.List()) ?? [];
    listElement.replaceChildren();
    listElement.hidden = reminders.length === 0;
    appElement.classList.toggle("is-empty", reminders.length === 0);
    document.querySelector(".new-button-label")!.textContent = reminders.length === 0 ? "Create Alarm" : "New";
    void refreshStartupWarning(reminders.length > 0);

    for (const reminder of [...reminders].reverse()) {
      const card = document.createElement("article");
      card.className = "reminder-card";
      card.tabIndex = 0;
      card.setAttribute("role", "button");
      card.setAttribute(
        "aria-label",
        `Edit reminder: ${reminder.name}, ${reminder.time}, ${formatReminderSchedule(reminder)}`,
      );
      card.addEventListener("click", () => openDialog(reminder));
      card.addEventListener("keydown", (event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          openDialog(reminder);
        }
      });

      const details = document.createElement("div");
      details.className = "reminder-details";

      const time = document.createElement("time");
      time.className = "reminder-card-time";
      time.dateTime = reminder.time;
      time.textContent = reminder.time;

      const title = document.createElement("h2");
      title.textContent = reminder.name;

      const schedule = document.createElement("p");
      schedule.textContent = formatReminderSchedule(reminder);

      details.append(title, schedule);
      card.append(time, details);
      listElement.append(card);
    }
  } catch (error) {
    showPageError(formatError(error));
  }
}

if (isPopupWindow) {
  Events.On("theme:changed", (event) => {
    if (event.data === "dark" || event.data === "light") {
      document.documentElement.dataset.theme = event.data;
    }
  });

  Events.On("reminder:due", (event) => {
    if (showDueReminder(event.data)) {
      queueNotificationSound(event.data.sound ?? "none");
    }
    void ReminderService.AcknowledgeAlert(event.data.id).catch((error) => {
      console.error("Could not acknowledge reminder:", error);
    });
  });

  void ReminderService.PendingAlerts()
    .then((reminders) => (reminders ?? []).forEach((reminder) => {
      if (showDueReminder(reminder)) {
        queueNotificationSound(reminder.sound ?? "none");
      }
    }))
    .catch((error) => console.error("Could not load pending reminders:", error));

  document.getElementById("dismiss-alert")!.addEventListener("click", async () => {
    try {
      await ReminderService.DismissAlert();
      popupRemindersElement.replaceChildren();
      popupAlertElement.hidden = true;
    } catch (error) {
      console.error("Could not dismiss reminder window:", error);
    }
  });
}

if (!isPopupWindow) {
  soundButtons.forEach((button) => {
    button.addEventListener("click", () => {
      const sound = button.dataset.soundId ?? "none";
      selectSound(sound);
      previewSound(sound);
      closeSoundMenu(true);
    });
    button.addEventListener("keydown", (event) => {
      const index = soundButtons.indexOf(button);
      if (event.key === "Escape") {
        event.preventDefault();
        closeSoundMenu(true);
      } else if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        event.preventDefault();
        const offset = event.key === "ArrowDown" ? 1 : -1;
        soundButtons[(index + offset + soundButtons.length) % soundButtons.length].focus();
      } else if (event.key === "Home") {
        event.preventDefault();
        soundButtons[0].focus();
      } else if (event.key === "End") {
        event.preventDefault();
        soundButtons[soundButtons.length - 1].focus();
      }
    });
  });
  soundSelectTrigger.addEventListener("click", () => {
    if (soundSelectMenu.hidden) {
      openSoundMenu();
    } else {
      closeSoundMenu();
    }
  });
  soundSelectTrigger.addEventListener("keydown", (event) => {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      openSoundMenu();
    }
  });
  document.addEventListener("pointerdown", (event) => {
    if (!soundSelect.contains(event.target as Node)) {
      closeSoundMenu();
    }
  });
  window.addEventListener("resize", () => {
    if (soundSelectMenu.matches(":popover-open")) {
      positionSoundMenu();
    }
  });
  dialog.addEventListener("scroll", () => {
    if (soundSelectMenu.matches(":popover-open")) {
      positionSoundMenu();
    }
  });

  updateThemeToggle();
  themeToggle.addEventListener("change", () => {
    setTheme(themeToggle.checked ? "dark" : "light");
  });
  document.getElementById("open-settings")!.addEventListener("click", () => {
    if (settingsCloseTimer) {
      clearTimeout(settingsCloseTimer);
    }
    settingsDialog.showModal();
    requestAnimationFrame(() => settingsDialog.classList.add("is-open"));
    void refreshStartupSetting();
  });
  startupAddButton.addEventListener("click", async () => {
    startupAddButton.disabled = true;
    try {
      await ReminderService.EnableStartup();
      await refreshStartupSetting();
    } catch (error) {
      startupStatus.textContent = formatError(error);
      startupAddButton.disabled = false;
    }
  });
  startupWarningAddButton.addEventListener("click", async () => {
    startupWarningAddButton.disabled = true;
    try {
      await ReminderService.EnableStartup();
      await refreshStartupWarning(true);
    } catch (error) {
      startupWarningMessage.textContent = `Could not add Alarme to Windows startup: ${formatError(error)}`;
    } finally {
      startupWarningAddButton.disabled = false;
    }
  });
  document.getElementById("close-settings")!.addEventListener("click", closeSettingsDialog);
  settingsDialog.addEventListener("click", (event) => {
    if (event.target === settingsDialog) {
      closeSettingsDialog();
    }
  });
  settingsDialog.addEventListener("cancel", (event) => {
    event.preventDefault();
    closeSettingsDialog();
  });
  Events.On("reminder:removed", () => {
    void loadReminders();
  });
  document.getElementById("new-reminder")!.addEventListener("click", () => openDialog());
  document.getElementById("close-dialog")!.addEventListener("click", closeDialog);
  document.getElementById("cancel-reminder")!.addEventListener("click", closeDialog);
  oneTimeInput.addEventListener("change", updateScheduleFields);
  document.getElementById("previous-month")!.addEventListener("click", () => {
    displayedMonth = new Date(displayedMonth.getFullYear(), displayedMonth.getMonth() - 1, 1);
    renderCalendar();
  });
  document.getElementById("next-month")!.addEventListener("click", () => {
    displayedMonth = new Date(displayedMonth.getFullYear(), displayedMonth.getMonth() + 1, 1);
    renderCalendar();
  });
  document.getElementById("today-date")!.addEventListener("click", () => {
    dateInput.value = localDateString(new Date());
    showCalendarForDate(dateInput.value);
    closeCalendar();
  });
  dateTrigger.addEventListener("click", () => {
    if (dateField.hidden) {
      openCalendar();
    } else {
      closeCalendar();
    }
  });
  dialog.addEventListener("scroll", () => {
    if (!dateField.hidden) {
      positionCalendar();
    }
  });
  window.addEventListener("resize", () => {
    if (!dateField.hidden) {
      positionCalendar();
    }
  });
  calendarDays.addEventListener("click", (event) => {
    if ((event.target as HTMLElement).closest(".calendar-day")) {
      closeCalendar();
    }
  });
  form.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && !dateField.hidden) {
      event.stopPropagation();
      closeCalendar();
      dateTrigger.focus();
    }
  });
  form.addEventListener("click", (event) => {
    if (!dateField.hidden && !datePickerWrap.contains(event.target as Node)) {
      closeCalendar();
    }
  });
  document.querySelectorAll<HTMLButtonElement>(".time-step").forEach((button) => {
    button.addEventListener("click", () => {
      const unit = button.dataset.unit;
      if (unit === "hour" || unit === "minute") {
        stepTime(unit, Number(button.dataset.step));
      }
    });
  });
  for (const [input, maximum] of [[hourInput, 23], [minuteInput, 59]] as const) {
    input.addEventListener("input", () => {
      const previous = Number(previousTimeValues.get(input));
      input.value = input.value.replace(/\D/g, "").slice(0, 2);
      const current = Number(input.value);
      if (
        input.value.length > 0 &&
        Number.isInteger(current) &&
        current >= 0 &&
        current <= maximum &&
        input.value !== previousTimeValues.get(input)
      ) {
        animateTimeChange(input, current >= previous ? 1 : -1);
        previousTimeValues.set(input, input.value);
      } else if (input.value.length > 0 && current > maximum) {
        input.value = previousTimeValues.get(input) ?? "00";
      }
      syncTimeValue();
    });
    input.addEventListener("blur", () => normalizeTimeInput(input, maximum));
    input.addEventListener("keydown", (event) => {
      if (event.key === "ArrowUp" || event.key === "ArrowDown") {
        event.preventDefault();
        stepTime(input === hourInput ? "hour" : "minute", event.key === "ArrowUp" ? 1 : -1);
      }
    });
    input.addEventListener(
      "wheel",
      (event) => {
        event.preventDefault();
        stepTime(input === hourInput ? "hour" : "minute", event.deltaY < 0 ? 1 : -1);
      },
      { passive: false },
    );
  }
  dialog.addEventListener("cancel", (event) => {
    event.preventDefault();
    closeDialog();
  });
  dialog.addEventListener("click", (event) => {
    if (event.target === dialog) {
      closeDialog();
    }
  });

  document.querySelectorAll<HTMLButtonElement>(".weekday").forEach((button) => {
    button.addEventListener("click", () => {
      const day = Number(button.dataset.day);
      if (selectedWeekdays.has(day)) {
        selectedWeekdays.delete(day);
        button.setAttribute("aria-pressed", "false");
      } else {
        selectedWeekdays.add(day);
        button.setAttribute("aria-pressed", "true");
      }
      formError.hidden = true;
    });
  });

  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    clearErrors();

    const type = oneTimeInput.checked ? "weekly" : "date";
    if (type === "weekly" && selectedWeekdays.size === 0) {
      formError.textContent = "Select at least one weekday.";
      formError.hidden = false;
      return;
    }
    if (type === "date" && !dateInput.value) {
      formError.textContent = "Select a date.";
      formError.hidden = false;
      return;
    }

    normalizeTimeInput(hourInput, 23);
    normalizeTimeInput(minuteInput, 59);
    saveButton.disabled = true;
    try {
      const input = {
        name: nameInput.value.trim() || "-",
        type,
        date: type === "date" ? dateInput.value : "",
        time: timeInput.value,
        sound: selectedSound,
        weekdays: type === "weekly" ? Array.from(selectedWeekdays).sort((a, b) => a - b) : [],
      };
      if (editingReminderId) {
        await ReminderService.Update(editingReminderId, input);
      } else {
        await ReminderService.Create(input);
      }
      closeDialog();
      await loadReminders();
    } catch (error) {
      formError.textContent = formatError(error);
      formError.hidden = false;
    } finally {
      saveButton.disabled = false;
    }
  });

  deleteButton.addEventListener("click", async () => {
    if (!editingReminderId) {
      return;
    }
    deleteButton.disabled = true;
    try {
      await ReminderService.Delete(editingReminderId);
      closeDialog();
      await loadReminders();
    } catch (error) {
      formError.textContent = formatError(error);
      formError.hidden = false;
    } finally {
      deleteButton.disabled = false;
    }
  });

  void loadReminders();
}
