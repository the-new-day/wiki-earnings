package discord

func (b *Bot) sendTextMessage(channelID string, message string) error {
	_, err := b.session.ChannelMessageSend(channelID, message)
	if err != nil {
		return err
	}
	return nil
}
