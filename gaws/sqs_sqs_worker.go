package gaws

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/maestre3d/gluon"
)

type snsSqsSubscriptionWorker struct {
	parentDriver *snsSqsDriver
	rootSub      *gluon.Subscriber
}

func newSnsSqsSubscriptionWorker(parent *snsSqsDriver) *snsSqsSubscriptionWorker {
	return &snsSqsSubscriptionWorker{parentDriver: parent}
}

func (s *snsSqsSubscriptionWorker) start(ctx context.Context, sub *gluon.Subscriber) error {
	s.rootSub = sub
	go func() {
		receiveTimes := 0
		failedPollingCount := 0
	subscriptionLoop:
		for {
			receiveTimes++
			queueUrl := generateSqsQueueUrl(s.parentDriver.config, s.getDefaultConsumerGroup(sub))
			out, err := s.parentDriver.sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
				QueueUrl:                aws.String(queueUrl),
				AttributeNames:          nil,
				MaxNumberOfMessages:     s.parentDriver.config.GetMaxNumberOfMessagesPolled(),
				MessageAttributeNames:   nil,
				ReceiveRequestAttemptId: nil,
				VisibilityTimeout:       s.parentDriver.config.GetVisibilityTimeout(),
				WaitTimeSeconds:         s.parentDriver.config.GetWaitTimeSeconds(),
			})
			s.logError(err)
			pollRetries := s.parentDriver.config.GetMaxBatchPollingRetries()
			willCountFail := pollRetries > 0 && failedPollingCount+1 >= pollRetries
			if err != nil && willCountFail {
				s.logError(errors.New("gluon: Failed to fetch from queue, stopping polling"))
				break
			} else if err != nil {
				failedPollingCount++
				time.Sleep(s.parentDriver.config.FailedPollingBackoff)
				continue
			}
			s.fanOutMessagesProcesses(out.Messages...)

			select {
			case <-ctx.Done():
				break subscriptionLoop
			default:
				continue
			}
		}
	}()
	return nil
}

func (s *snsSqsSubscriptionWorker) logError(err error) {
	if err == nil {
		return
	}

	if s.parentDriver.parentBus.Logger != nil && s.parentDriver.isLoggingEnabled() {
		s.parentDriver.parentBus.Logger.Print(err)
	}
}

func (s *snsSqsSubscriptionWorker) getDefaultConsumerGroup(sub *gluon.Subscriber) string {
	if group := sub.GetGroup(); group != "" {
		return group
	}
	return s.parentDriver.parentBus.Configuration.ConsumerGroup
}

func (s *snsSqsSubscriptionWorker) fanOutMessagesProcesses(msgs ...types.Message) {
	for _, msg := range msgs {
		go s.processMessage(msg)
	}
}

func (s *snsSqsSubscriptionWorker) processMessage(snsMessage types.Message) {
	gluonMsg, err := unmarshalSnsMessage(snsMessage.Body)
	s.logError(err)
	if err != nil {
		return
	}
	go s.execMessageHandler(snsMessage, gluonMsg, s.rootSub)
}

func (s *snsSqsSubscriptionWorker) execMessageHandler(snsMessage types.Message, msg *gluon.TransportMessage,
	sub *gluon.Subscriber) {
	// A. If processing succeed, remove message from queue; AWS SQS will consider this action as a
	// successful processing.
	//
	// B. If processing failed, do nothing; AWS SQS Queue should be configured with a re-drive policy to a
	// Dead-Letter queue (DLQ) when a delivery count is equal to a factor specified by the developer.
	// Nevertheless, the developer should be aware of the VisibilityTimeout factor as AWS SQS uses it to re-deliver
	// messages to other subscribers/pollers.
	//
	// For more information, look here:
	// https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-visibility-timeout.html#inflight-messages
	scopedCtx := context.Background()
	queueUrl := aws.String(generateSqsQueueUrl(s.parentDriver.config, s.getDefaultConsumerGroup(sub)))
	err := s.parentDriver.messageHandler(scopedCtx, sub, msg)
	s.logError(err)
	if err != nil {
		return
	}

	_, err = s.parentDriver.sqsClient.DeleteMessage(scopedCtx, &sqs.DeleteMessageInput{
		QueueUrl:      queueUrl,
		ReceiptHandle: snsMessage.ReceiptHandle,
	})
	s.logError(err)
}
