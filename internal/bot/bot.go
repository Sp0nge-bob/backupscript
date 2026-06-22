package bot

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/Sp0nge-bob/backupscript/internal/backup"
	"github.com/Sp0nge-bob/backupscript/internal/config"
	"github.com/Sp0nge-bob/backupscript/internal/interval"
	"github.com/Sp0nge-bob/backupscript/internal/scheduler"
)

type LastBackup struct {
	Time    time.Time
	Size    int64
	Path    string
	Success bool
	Error   string
}

type Service struct {
	api        *tgbotapi.BotAPI
	cfg        *config.Config
	sched      *scheduler.Manager
	mu         sync.RWMutex
	lastBackup LastBackup
}

func New(api *tgbotapi.BotAPI, cfg *config.Config) *Service {
	return &Service{api: api, cfg: cfg}
}

func (s *Service) SetScheduler(sched *scheduler.Manager) {
	s.sched = sched
}

func (s *Service) Run() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := s.api.GetUpdatesChan(u)
	log.Printf("bot started as @%s", s.api.Self.UserName)

	for update := range updates {
		if update.CallbackQuery != nil {
			s.handleCallback(update.CallbackQuery)
			continue
		}
		if update.Message == nil || !update.Message.IsCommand() {
			continue
		}
		s.handleCommand(update.Message)
	}
}

func (s *Service) handleCommand(msg *tgbotapi.Message) {
	if !s.cfg.IsAllowed(msg.From.ID) {
		s.sendText(msg.Chat.ID, "Доступ запрещён.")
		return
	}

	switch msg.Command() {
	case "start":
		text := "Бот бекапов сервера.\n\nКоманды:\n/backup — создать и отправить архив\n/schedule — интервал автобекапа (30m, 6h, 7d)\n/list — пути и расписание\n/status — статус\n/help — справка"
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Сделать бекап", "backup"),
			),
		)
		reply := tgbotapi.NewMessage(msg.Chat.ID, text)
		reply.ReplyMarkup = keyboard
		if _, err := s.api.Send(reply); err != nil {
			log.Printf("send start: %v", err)
		}
	case "backup":
		s.runBackup(msg.Chat.ID, msg.From.ID)
	case "list":
		s.sendList(msg.Chat.ID)
	case "status":
		s.sendStatus(msg.Chat.ID)
	case "schedule":
		s.handleSchedule(msg.Chat.ID, strings.TrimSpace(msg.CommandArguments()))
	case "help":
		s.sendText(msg.Chat.ID, "Команды:\n/backup — архив и отправка\n/schedule — автобекап: /schedule 6h, /schedule off\n/list — пути бекапа и интервал\n/status — последний бекап\n/help — эта справка\n\nИнтервал: 30m, 6h, 7d, 1w (минимум 1m).\nПути задаются в config.yaml.")
	default:
		s.sendText(msg.Chat.ID, "Неизвестная команда. /help")
	}
}

func (s *Service) handleCallback(cb *tgbotapi.CallbackQuery) {
	callback := tgbotapi.NewCallback(cb.ID, "")
	if !s.cfg.IsAllowed(cb.From.ID) {
		callback.Text = "Доступ запрещён"
		_, _ = s.api.Request(callback)
		return
	}

	if cb.Data == "backup" {
		_, _ = s.api.Request(callback)
		s.runBackup(cb.Message.Chat.ID, cb.From.ID)
		return
	}

	_, _ = s.api.Request(callback)
}

func (s *Service) runBackup(chatID, userID int64) {
	s.sendText(chatID, "Создаю бекап...")

	result, err := s.CreateBackup()
	if err != nil {
		s.setLastBackup(LastBackup{Time: time.Now(), Success: false, Error: err.Error()})
		s.sendText(chatID, "Ошибка: "+err.Error())
		log.Printf("backup failed for user %d: %v", userID, err)
		return
	}
	defer func() {
		if err := os.Remove(result.Path); err != nil {
			log.Printf("remove archive %s: %v", result.Path, err)
		}
	}()

	caption := fmt.Sprintf("Бекап %s (%s)", result.CreatedAt.Format("2006-01-02 15:04:05"), backup.FormatSize(result.Size))
	if len(result.Warnings) > 0 {
		caption += "\n\nПредупреждения:\n" + strings.Join(result.Warnings, "\n")
	}

	doc := tgbotapi.NewDocument(chatID, tgbotapi.FilePath(result.Path))
	doc.Caption = caption
	if _, err := s.api.Send(doc); err != nil {
		s.setLastBackup(LastBackup{Time: time.Now(), Success: false, Error: err.Error()})
		s.sendText(chatID, "Архив создан, но отправка не удалась: "+err.Error())
		log.Printf("send document: %v", err)
		return
	}

	s.setLastBackup(LastBackup{
		Time:    result.CreatedAt,
		Size:    result.Size,
		Path:    result.Path,
		Success: true,
	})
	log.Printf("backup sent to chat %d (%s)", chatID, backup.FormatSize(result.Size))
}

func (s *Service) CreateBackup() (*backup.Result, error) {
	return backup.Create(backup.Config{
		Name:    s.cfg.Backup.Name,
		Paths:   s.cfg.Backup.Paths,
		Exclude: s.cfg.Backup.Exclude,
		TmpDir:  s.cfg.TmpDir,
	})
}

func (s *Service) SendBackupTo(chatID int64) error {
	result, err := s.CreateBackup()
	if err != nil {
		s.setLastBackup(LastBackup{Time: time.Now(), Success: false, Error: err.Error()})
		return err
	}
	defer func() {
		if err := os.Remove(result.Path); err != nil {
			log.Printf("remove archive %s: %v", result.Path, err)
		}
	}()

	caption := fmt.Sprintf("Плановый бекап %s (%s)", result.CreatedAt.Format("2006-01-02 15:04:05"), backup.FormatSize(result.Size))
	if len(result.Warnings) > 0 {
		caption += "\n\nПредупреждения:\n" + strings.Join(result.Warnings, "\n")
	}

	doc := tgbotapi.NewDocument(chatID, tgbotapi.FilePath(result.Path))
	doc.Caption = caption
	if _, err := s.api.Send(doc); err != nil {
		s.setLastBackup(LastBackup{Time: time.Now(), Success: false, Error: err.Error()})
		return fmt.Errorf("send document: %w", err)
	}

	s.setLastBackup(LastBackup{
		Time:    result.CreatedAt,
		Size:    result.Size,
		Path:    result.Path,
		Success: true,
	})
	return nil
}

func (s *Service) sendList(chatID int64) {
	statuses := backup.InspectPaths(s.cfg.Backup.Paths)
	var b strings.Builder
	b.WriteString("Настройки из config.yaml:\n\n")
	b.WriteString(fmt.Sprintf("Автобекап: %s\n\n", s.cfg.Schedule.ScheduleDescription()))
	b.WriteString("Пути бекапа:\n")
	for _, st := range statuses {
		state := "ok"
		if !st.Exists {
			state = "missing"
		} else if st.IsDir {
			state = "dir"
		}
		b.WriteString(fmt.Sprintf("%s — %s\n", state, st.Path))
	}
	s.sendText(chatID, b.String())
}

func (s *Service) handleSchedule(chatID int64, arg string) {
	arg = strings.TrimSpace(strings.ToLower(arg))

	if arg == "" {
		s.sendText(chatID, fmt.Sprintf(
			"Автобекап: %s\n\nЗадать интервал:\n/schedule 30m\n/schedule 6h\n/schedule 7d\n\nВыключить: /schedule off\nВключить с текущим интервалом: /schedule on",
			s.cfg.Schedule.ScheduleDescription(),
		))
		return
	}

	schedule := s.cfg.Schedule

	switch arg {
	case "off":
		schedule.Enabled = false
	case "on":
		if !schedule.UsesInterval() && schedule.Cron == "" {
			s.sendText(chatID, "Сначала задайте интервал: /schedule 6h")
			return
		}
		schedule.Enabled = true
	default:
		if _, err := interval.Parse(arg); err != nil {
			s.sendText(chatID, "Ошибка: "+err.Error())
			return
		}
		schedule.Interval = arg
		schedule.Enabled = true
	}

	if err := s.cfg.SaveSchedule(schedule); err != nil {
		s.sendText(chatID, "Не удалось сохранить config.yaml: "+err.Error())
		return
	}

	if s.sched == nil {
		s.sendText(chatID, "Сохранено. Перезапустите бота для применения.")
		return
	}

	if err := s.sched.Reload(s.cfg); err != nil {
		s.sendText(chatID, "Сохранено, но перезапуск расписания не удался: "+err.Error())
		return
	}

	s.sendText(chatID, fmt.Sprintf("Автобекап: %s", schedule.ScheduleDescription()))
}

func (s *Service) sendStatus(chatID int64) {
	s.mu.RLock()
	last := s.lastBackup
	s.mu.RUnlock()

	var b strings.Builder
	b.WriteString("Статус бота\n\n")
	b.WriteString(fmt.Sprintf("Автобекап: %s\n", s.cfg.Schedule.ScheduleDescription()))

	if last.Time.IsZero() {
		b.WriteString("Последний бекап: ещё не выполнялся\n")
	} else if last.Success {
		b.WriteString(fmt.Sprintf("Последний бекап: %s, %s\n", last.Time.Format("2006-01-02 15:04:05"), backup.FormatSize(last.Size)))
	} else {
		b.WriteString(fmt.Sprintf("Последний бекап: ошибка в %s\n%s\n", last.Time.Format("2006-01-02 15:04:05"), last.Error))
	}

	s.sendText(chatID, b.String())
}

func (s *Service) sendText(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := s.api.Send(msg); err != nil {
		log.Printf("send message: %v", err)
	}
}

func (s *Service) setLastBackup(info LastBackup) {
	s.mu.Lock()
	s.lastBackup = info
	s.mu.Unlock()
}

func (s *Service) API() *tgbotapi.BotAPI {
	return s.api
}

