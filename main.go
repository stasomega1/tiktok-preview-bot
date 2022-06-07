package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/Jeffail/gabs/v2"
	"github.com/PuerkitoBio/goquery"
	"github.com/avast/retry-go"
	"github.com/caarlos0/env/v6"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/chromedp"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	http "github.com/zMrKrabz/fhttp"
	"log"
	"regexp"
	"strings"
	"time"
)

var (
	ErrConn  = errors.New("connection error")
	ErrEmpty = errors.New("video not found")
)

const (
	version = "1.0.7"
)

func init() {
	fmt.Printf("Version: %s\n", version)
}

type TgBot struct {
	ApiKey string `env:"BOT_API_KEY"`
}

func main() {
	tgBot := TgBot{}
	err := env.Parse(&tgBot)
	if err != nil || tgBot.ApiKey == "" {
		panic(err)
	}
	bot, err := tgbotapi.NewBotAPI(tgBot.ApiKey)
	if err != nil {
		panic(err)
	}

	updChan := bot.GetUpdatesChan(tgbotapi.UpdateConfig{
		Offset:         0,
		Limit:          1,
		Timeout:        1,
		AllowedUpdates: nil,
	})

	for val := range updChan {
		if val.Message == nil {
			continue
		}

		r := regexp.MustCompile(`(http|https):\/\/([\w_-]+(?:(?:\.[\w_-]+)+))([\w.,@?^=%&:\/~+#-]*[\w@?^=%&\/~+#-])`)
		url := r.FindString(val.Message.Text)
		if url != "" && strings.Contains(url, "tiktok.com") {
			endChan := make(chan interface{})
			messageIdChan := make(chan int)
			go SendWaitingMessage(bot, val.FromChat().ID, val.Message.MessageID, endChan, messageIdChan)
			videoUrl, err := GetVideoUrl(url)
			if err != nil {
				fmt.Printf("Message: %s, error: %s\n", val.Message.Text, err)
				break
			}
			fmt.Printf("Текст: %s, ссылка: %s\n", val.Message.Text, videoUrl)

			fileUrl := tgbotapi.FileURL(videoUrl)
			mediaVideo := tgbotapi.NewInputMediaVideo(fileUrl)
			mediaText := fmt.Sprintf("Sender: @%s\nOriginal message: %s", val.Message.From.UserName, val.Message.Text)
			mediaVideo.Caption = mediaText
			mediaGroup := tgbotapi.NewMediaGroup(val.FromChat().ID, []interface{}{mediaVideo})
			mediaGroup.DisableNotification = true
			message, err := bot.SendMediaGroup(mediaGroup)
			message = message

			deleteMessage := tgbotapi.NewDeleteMessage(val.FromChat().ID, val.Message.MessageID)
			resp, err := bot.Request(deleteMessage)
			resp = resp

			endChan <- 1
			DeleteWaitingMessage(bot, val.FromChat().ID, messageIdChan)

			//m := tgbotapi.NewMessage(val.FromChat().ID, videoUrl)
			//bot.Send(m)
		}
	}
}

func SendWaitingMessage(api *tgbotapi.BotAPI, chatId int64, replyMessageId int, endChan <-chan interface{},
	postMessageIdChan chan<- int) {
	text := "Video loading in progress"
	preMessage := tgbotapi.NewMessage(chatId, text)
	preMessage.DisableNotification = true
	preMessage.ReplyToMessageID = replyMessageId
	postMessage, err := api.Send(preMessage)
	if err != nil {
		fmt.Println(err)
		return
	}

	dots := "."
	for {
		isBreak := false
		preEdit := tgbotapi.NewEditMessageText(chatId, postMessage.MessageID, text+dots)
		time.Sleep(1 * time.Second)
		_, err = api.Request(preEdit)
		if err != nil {
			fmt.Println(err)
		}

		select {
		case <-endChan:
			isBreak = true
		default:
		}

		if isBreak {
			break
		}

		dots = dots + "."
		if len(dots) > 3 {
			dots = ""
		}
	}

	postMessageIdChan <- postMessage.MessageID
}

func DeleteWaitingMessage(api *tgbotapi.BotAPI, chatId int64, messageIdChan <-chan int) {
	preDeleteMessage := tgbotapi.NewDeleteMessage(chatId, <-messageIdChan)
	_, err := api.Request(preDeleteMessage)
	if err != nil {
		fmt.Println(err)
	}
}

func GetVideoUrl(url string) (string, error) {
	resultChan := make(chan string, 2)
	err := retry.Do(func() error {
		result, err := getVideoUrlChrome(url)
		if err != nil {
			return err
		}
		resultChan <- result
		return nil
	},
		retry.Attempts(5))
	if err != nil {
		return "", ErrConn
	}

	video := <-resultChan

	if err != nil {
		return "", ErrEmpty
	}

	return video, nil
}

func getVideoUrlHttpClient(url string) (string, error) {
	fmt.Println(1)
	client := &http.Client{}
	client.Timeout = 10 * time.Second

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println(2)
		fmt.Println(err)
		return "", fmt.Errorf("http.NewRequest: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/81.0.4044.138 Safari/537.36")
	req.Header.Set("Host", "vt.tiktok.com")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	fmt.Println(3)
	res, err := client.Do(req)
	if err != nil {
		fmt.Println(4)
		fmt.Println(err)
		return "", fmt.Errorf("client.Do: %v", err)
	}
	defer res.Body.Close()
	fmt.Println(5)
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		fmt.Println(6)
		fmt.Println(err)
		return "", fmt.Errorf("goquery.NewDocumentFromReader: %v", err)
	}
	fmt.Println(7)
	fmt.Println(doc.Text())
	fmt.Println(8)
	text := doc.Find("#SIGI_STATE").Text()
	fmt.Println(text)
	fmt.Println(9)
	result, err := gabs.ParseJSON([]byte(text))
	if err != nil {
		fmt.Println(10)
		fmt.Println(err)
		return "", fmt.Errorf("gabs.ParseJSON: %v", err)
	}
	fmt.Println(11)

	result = result.Path("ItemList.video.preloadList.0.url")
	finalUrl := string(result.EncodeJSON())
	finalUrl = strings.TrimLeft(finalUrl, `["`)
	finalUrl = strings.TrimRight(finalUrl, `"]`)
	finalUrl = strings.Split(finalUrl, "?")[0]
	fmt.Println(12)
	return finalUrl, nil
}

func getVideoUrlChrome(url string) (string, error) {
	// create chrome instance
	ctx, cancel := chromedp.NewContext(
		context.Background(),
		// chromedp.WithDebugf(log.Printf),
	)
	ctx, cancel = context.WithTimeout(ctx, time.Second*30)
	defer cancel()
	body := ""
	// navigate to a page, wait for an element, click
	err := chromedp.Run(ctx,
		//chromedp.Navigate(`https://pkg.go.dev/time`),
		chromedp.Navigate(url),
		chromedp.ActionFunc(func(ctx context.Context) error {
			node, err := dom.GetDocument().Do(ctx)
			if err != nil {
				return err
			}
			body, err = dom.GetOuterHTML().WithNodeID(node.NodeID).Do(ctx)
			return err
		}),
	)
	if err != nil {
		log.Println(err)
		return "", err
	}

	r := regexp.MustCompile(`(https):\/\/v16-webapp\.([\w_-]+(?:(?:\.[\w_-]+)+))([\w.,@?^=%&:\/~+#-]*[\w@?^=%&\/~+#-])`)
	result := r.FindAllString(body, 1)
	if len(result) < 1 {
		return "", errors.New("ни смог :(")
	}

	return result[0], nil
}
