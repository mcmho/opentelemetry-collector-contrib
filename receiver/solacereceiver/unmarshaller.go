// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	model_v1 "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/model/v1"
)

// tracesUnmarshaller deserializes the message body.
type tracesUnmarshaller interface {
	// unmarshal the amqp-message into traces.
	// Only valid traces are produced or error is returned
	unmarshal(message *inboundMessage) (*ptrace.Traces, error)
}

// newUnmarshalleer returns a new unmarshaller ready for message unmarshalling
func newTracesUnmarshaller(logger *zap.Logger) tracesUnmarshaller {
	return &solaceTracesUnmarshaller{
		logger: logger,
		// v1 unmarshaller is implemented by solaceMessageUnmarshallerV1
		v1: &solaceMessageUnmarshallerV1{
			logger: logger,
		},
	}
}

// solaceTracesUnmarshaller implements tracesUnmarshaller.
type solaceTracesUnmarshaller struct {
	logger *zap.Logger
	v1     tracesUnmarshaller
}

var (
	errUnknownTraceMessgeVersion = errors.New("unsupported trace message version")
	errUnknownTraceMessgeType    = errors.New("bad trace message")
)

// unmarshal will unmarshal an *solaceMessage into *ptrace.Traces.
// It will make a decision based on the version of the message which unmarshalling strategy to use.
// For now, only v1 messages are used.
func (u *solaceTracesUnmarshaller) unmarshal(message *inboundMessage) (*ptrace.Traces, error) {
	const (
		topicPrefix   = "_telemetry/broker/trace/receive/v"
		topicPrefixV1 = topicPrefix + "1"
	)
	if message.Properties != nil && message.Properties.To != nil {
		if strings.HasPrefix(*message.Properties.To, topicPrefixV1) {
			return u.v1.unmarshal(message)
		}
		if strings.HasPrefix(*message.Properties.To, topicPrefix) {
			// unknown version
			u.logger.Error("Received message with unsupported version topic", zap.String("topic", *message.Properties.To))
			return nil, errUnknownTraceMessgeVersion
		}
		// unknown topic
		u.logger.Error("Received message with unknown topic", zap.String("topic", *message.Properties.To))
		return nil, errUnknownTraceMessgeType
	}
	// no topic
	u.logger.Error("Received message with no topic")
	return nil, errUnknownTraceMessgeType
}

type solaceMessageUnmarshallerV1 struct {
	logger *zap.Logger
}

// unmarshal implements tracesUnmarshaller.unmarshal
func (u *solaceMessageUnmarshallerV1) unmarshal(message *inboundMessage) (*ptrace.Traces, error) {
	spanData, err := u.unmarshalToSpanData(message)
	if err != nil {
		return nil, err
	}
	traces := ptrace.NewTraces()
	u.populateTraces(spanData, &traces)
	return &traces, nil
}

// unmarshalToSpanData will consume an solaceMessage and unmarshal it into a SpanData.
// Returns an error if one occurred.
func (u *solaceMessageUnmarshallerV1) unmarshalToSpanData(message *inboundMessage) (*model_v1.SpanData, error) {
	var spanData model_v1.SpanData
	if err := proto.Unmarshal(message.GetData(), &spanData); err != nil {
		return nil, err
	}
	return &spanData, nil
}

// createSpan will create a new Span from the given traces and map the given SpanData to the span.
// This will set all required fields such as name version, trace and span ID, parent span ID (if applicable),
// timestamps, errors and states.
func (u *solaceMessageUnmarshallerV1) populateTraces(spanData *model_v1.SpanData, traces *ptrace.Traces) {
	// Append new resource span and map any attributes
	resourceSpan := traces.ResourceSpans().AppendEmpty()
	resourceSpanAttributes := resourceSpan.Resource().Attributes()
	u.mapResourceSpanAttributes(spanData, &resourceSpanAttributes)
	instrLibrarySpans := resourceSpan.ScopeSpans().AppendEmpty()
	// Create a new span
	clientSpan := instrLibrarySpans.Spans().AppendEmpty()
	// map the basic span data
	u.mapClientSpanData(spanData, &clientSpan)
	// map all span attributes
	clientSpanAttributes := clientSpan.Attributes()
	u.mapClientSpanAttributes(spanData, &clientSpanAttributes)
	// map all events
	u.mapEvents(spanData, &clientSpan)
}

func (u *solaceMessageUnmarshallerV1) mapResourceSpanAttributes(spanData *model_v1.SpanData, attrMap *pcommon.Map) {
	const (
		routerNameAttrKey     = "service.name"
		messageVpnNameAttrKey = "service.instance.id"
		solosVersionAttrKey   = "service.version"
	)
	if spanData.RouterName != nil {
		attrMap.InsertString(routerNameAttrKey, *spanData.RouterName)
	}
	if spanData.MessageVpnName != nil {
		attrMap.InsertString(messageVpnNameAttrKey, *spanData.MessageVpnName)
	}
	attrMap.InsertString(solosVersionAttrKey, spanData.SolosVersion)
}

func (u *solaceMessageUnmarshallerV1) mapClientSpanData(spanData *model_v1.SpanData, clientSpan *ptrace.Span) {
	const clientSpanName = "(topic) receive"

	// client span constants
	clientSpan.SetName(clientSpanName)
	// SPAN_KIND_CONSUMER == 5
	clientSpan.SetKind(5)

	// map trace ID
	var traceID [16]byte
	copy(traceID[:16], spanData.TraceId)
	clientSpan.SetTraceID(pcommon.NewTraceID(traceID))
	// map span ID
	var spanID [8]byte
	copy(spanID[:8], spanData.SpanId)
	clientSpan.SetSpanID(pcommon.NewSpanID(spanID))
	// conditional parent-span-id
	if len(spanData.ParentSpanId) == 8 {
		var parentSpanID [8]byte
		copy(parentSpanID[:8], spanData.ParentSpanId)
		clientSpan.SetParentSpanID(pcommon.NewSpanID(parentSpanID))
	}

	// timestamps
	clientSpan.SetStartTimestamp(pcommon.Timestamp(spanData.GetStartTimeUnixNano()))
	clientSpan.SetEndTimestamp(pcommon.Timestamp(spanData.GetEndTimeUnixNano()))
	// status
	if spanData.ErrorDescription != "" {
		clientSpan.Status().SetCode(ptrace.StatusCodeError)
		clientSpan.Status().SetMessage(spanData.ErrorDescription)
	}
	// trace state
	if spanData.TraceState != nil {
		clientSpan.SetTraceState(ptrace.TraceState(*spanData.TraceState))
	}
}

// mapAttributes takes a set of attributes from SpanData and maps them to ClientSpan.Attributes().
// Will also copy any user properties stored in the SpanData with a best effort approach.
func (u *solaceMessageUnmarshallerV1) mapClientSpanAttributes(spanData *model_v1.SpanData, attrMap *pcommon.Map) {
	// constant attributes
	const (
		systemAttrKey      = "messaging.system"
		systemAttrValue    = "SolacePubSub+"
		operationAttrKey   = "messaging.operation"
		operationAttrValue = "receive"
	)
	attrMap.InsertString(systemAttrKey, systemAttrValue)
	attrMap.InsertString(operationAttrKey, operationAttrValue)
	// attributes from spanData
	const (
		protocolAttrKey                    = "messaging.protocol"
		protocolVersionAttrKey             = "messaging.protocol_version"
		messageIDAttrKey                   = "messaging.message_id"
		conversationIDAttrKey              = "messaging.conversation_id"
		payloadSizeBytesAttrKey            = "messaging.message_payload_size_bytes"
		destinationAttrKey                 = "messaging.destination"
		clientUsernameAttrKey              = "messaging.solace.client_username"
		clientNameAttrKey                  = "messaging.solace.client_name"
		replicationGroupMessageIDAttrKey   = "messaging.solace.replication_group_message_id"
		priorityAttrKey                    = "messaging.solace.priority"
		ttlAttrKey                         = "messaging.solace.ttl"
		dmqEligibleAttrKey                 = "messaging.solace.dmq_eligible"
		droppedEnqueueEventsSuccessAttrKey = "messaging.solace.dropped_enqueue_events_success"
		droppedEnqueueEventsFailedAttrKey  = "messaging.solace.dropped_enqueue_events_failed"
		replyToAttrKey                     = "messaging.solace.reply_to_topic"
		receiveTimeAttrKey                 = "messaging.solace.broker_receive_time_unix_nano"
		droppedUserPropertiesAttrKey       = "messaging.solace.dropped_user_properties"
		hostIPAttrKey                      = "net.host.ip"
		hostPortAttrKey                    = "net.host.port"
		peerIPAttrKey                      = "net.peer.ip"
		peerPortAttrKey                    = "net.peer.port"
		userPropertiesPrefixAttrKey        = "messaging.solace.user_properties."
	)
	attrMap.InsertString(protocolAttrKey, spanData.Protocol)
	if spanData.ProtocolVersion != nil {
		attrMap.InsertString(protocolVersionAttrKey, *spanData.ProtocolVersion)
	}
	if spanData.ApplicationMessageId != nil {
		attrMap.InsertString(messageIDAttrKey, *spanData.ApplicationMessageId)
	}
	if spanData.CorrelationId != nil {
		attrMap.InsertString(conversationIDAttrKey, *spanData.CorrelationId)
	}
	attrMap.InsertInt(payloadSizeBytesAttrKey, int64(spanData.BinaryAttachmentSize+spanData.XmlAttachmentSize+spanData.MetadataSize))
	attrMap.InsertString(clientUsernameAttrKey, spanData.ClientUsername)
	attrMap.InsertString(clientNameAttrKey, spanData.ClientName)
	attrMap.InsertInt(receiveTimeAttrKey, spanData.BrokerReceiveTimeUnixNano)
	attrMap.InsertString(destinationAttrKey, spanData.Topic)

	rgmid := u.rgmidToString(spanData.ReplicationGroupMessageId)
	if len(rgmid) > 0 {
		attrMap.InsertString(replicationGroupMessageIDAttrKey, rgmid)
	}

	if spanData.Priority != nil {
		attrMap.InsertInt(priorityAttrKey, int64(*spanData.Priority))
	}
	if spanData.Ttl != nil {
		attrMap.InsertInt(ttlAttrKey, *spanData.Ttl)
	}
	if spanData.ReplyToTopic != nil {
		attrMap.InsertString(replyToAttrKey, *spanData.ReplyToTopic)
	}
	attrMap.InsertBool(dmqEligibleAttrKey, spanData.DmqEligible)
	attrMap.InsertInt(droppedEnqueueEventsSuccessAttrKey, int64(spanData.DroppedEnqueueEventsSuccess))
	attrMap.InsertInt(droppedEnqueueEventsFailedAttrKey, int64(spanData.DroppedEnqueueEventsFailed))

	hostIPLen := len(spanData.HostIp)
	if hostIPLen == 4 || hostIPLen == 16 {
		attrMap.InsertString(hostIPAttrKey, net.IP(spanData.HostIp).String())
	} else {
		u.logger.Warn("Host ip attribute has an illegal length", zap.Int("length", hostIPLen))
		recordRecoverableUnmarshallingError()
	}
	attrMap.InsertInt(hostPortAttrKey, int64(spanData.HostPort))

	peerIPLen := len(spanData.HostIp)
	if peerIPLen == 4 || peerIPLen == 16 {
		attrMap.InsertString(peerIPAttrKey, net.IP(spanData.PeerIp).String())
	} else {
		u.logger.Warn("Peer ip attribute has an illegal length", zap.Int("length", peerIPLen))
		recordRecoverableUnmarshallingError()
	}
	attrMap.InsertInt(peerPortAttrKey, int64(spanData.PeerPort))

	attrMap.InsertBool(droppedUserPropertiesAttrKey, spanData.DroppedUserProperties)
	for key, value := range spanData.UserProperties {
		if value != nil {
			u.insertUserProperty(attrMap, key, value.Value)
		}
	}
}

// mapEvents maps all events contained in SpanData to relevant events within clientSpan.Events()
func (u *solaceMessageUnmarshallerV1) mapEvents(spanData *model_v1.SpanData, clientSpan *ptrace.Span) {
	// handle enqueue events
	for _, enqueueEvent := range spanData.EnqueueEvents {
		u.mapEnqueueEvent(enqueueEvent, clientSpan)
	}

	// handle transaction events
	if transactionEvent := spanData.TransactionEvent; transactionEvent != nil {
		u.mapTransactionEvent(transactionEvent, clientSpan)
	}
}

// mapEnqueueEvent maps a SpanData_EnqueueEvent to a ClientSpan.Event
func (u *solaceMessageUnmarshallerV1) mapEnqueueEvent(enqueueEvent *model_v1.SpanData_EnqueueEvent, clientSpan *ptrace.Span) {
	const (
		enqueueEventSuffix               = " enqueue" // Final should be `<dest> enqueue`
		messagingDestinationEventKey     = "messaging.destination"
		messagingDestinationTypeEventKey = "messaging.solace.destination_type"
		statusMessageEventKey            = "messaging.solace.enqueue_error_message"
		rejectsAllEnqueuesKey            = "messaging.solace.rejects_all_enqueues"
		queueKind                        = "queue"
		topicEndpointKind                = "topic-endpoint"
		anonymousQueuePrefix             = "#P2P"
		anonymousQueueEventName          = "(anonymous)" + enqueueEventSuffix
	)
	var destinationName string
	var destinationType string
	switch casted := enqueueEvent.Dest.(type) {
	case *model_v1.SpanData_EnqueueEvent_TopicEndpointName:
		destinationName = casted.TopicEndpointName
		destinationType = topicEndpointKind
	case *model_v1.SpanData_EnqueueEvent_QueueName:
		destinationName = casted.QueueName
		destinationType = queueKind
	default:
		u.logger.Warn(fmt.Sprintf("Unknown destination type %T", casted))
		recordRecoverableUnmarshallingError()
		return
	}
	clientEvent := clientSpan.Events().AppendEmpty()
	var eventName string
	if strings.HasPrefix(destinationName, anonymousQueuePrefix) {
		eventName = anonymousQueueEventName
	} else {
		eventName = destinationName + enqueueEventSuffix
	}
	clientEvent.SetName(eventName)
	clientEvent.SetTimestamp(pcommon.Timestamp(enqueueEvent.TimeUnixNano))
	clientEvent.Attributes().InsertString(messagingDestinationEventKey, destinationName)
	clientEvent.Attributes().InsertString(messagingDestinationTypeEventKey, destinationType)
	clientEvent.Attributes().InsertBool(rejectsAllEnqueuesKey, enqueueEvent.RejectsAllEnqueues)
	if enqueueEvent.ErrorDescription != nil {
		clientEvent.Attributes().InsertString(statusMessageEventKey, enqueueEvent.GetErrorDescription())
	}
}

// mapTransactionEvent maps a SpanData_TransactionEvent to a ClientSpan.Event
func (u *solaceMessageUnmarshallerV1) mapTransactionEvent(transactionEvent *model_v1.SpanData_TransactionEvent, clientSpan *ptrace.Span) {
	const (
		transactionInitiatorEventKey    = "messaging.solace.transaction_initiator"
		transactionIDEventKey           = "messaging.solace.transaction_id"
		transactedSessionNameEventKey   = "messaging.solace.transacted_session_name"
		transactedSessionIDEventKey     = "messaging.solace.transacted_session_id"
		transactionErrorMessageEventKey = "messaging.solace.transaction_error_message"
		transactionXIDEventKey          = "messaging.solace.transaction_xid"
	)
	// map the transaction type to a name
	var name string
	switch transactionEvent.GetType() {
	case model_v1.SpanData_TransactionEvent_COMMIT:
		name = "commit"
	case model_v1.SpanData_TransactionEvent_ROLLBACK:
		name = "rollback"
	case model_v1.SpanData_TransactionEvent_END:
		name = "end"
	case model_v1.SpanData_TransactionEvent_PREPARE:
		name = "prepare"
	default:
		u.logger.Warn(fmt.Sprintf("Unknown transaction type %d", transactionEvent.GetType()))
		recordRecoverableUnmarshallingError()
		return // exit when we don't have a valid type since we should not add a span without a name
	}
	clientEvent := clientSpan.Events().AppendEmpty()
	clientEvent.SetName(name)
	clientEvent.SetTimestamp(pcommon.Timestamp(transactionEvent.TimeUnixNano))
	// map initiator enums to expected initiator strings
	var initiator string
	switch transactionEvent.GetInitiator() {
	case model_v1.SpanData_TransactionEvent_CLIENT:
		initiator = "client"
	case model_v1.SpanData_TransactionEvent_ADMIN:
		initiator = "administrator"
	case model_v1.SpanData_TransactionEvent_SESSION_TIMEOUT:
		initiator = "session timeout"
	default:
		u.logger.Warn(fmt.Sprintf("Unknown transaction initiator %d", transactionEvent.GetInitiator()))
		recordRecoverableUnmarshallingError()
	}
	clientEvent.Attributes().InsertString(transactionInitiatorEventKey, initiator)
	// conditionally set the error description if one occurred, otherwise omit
	if transactionEvent.ErrorDescription != nil {
		clientEvent.Attributes().InsertString(transactionErrorMessageEventKey, transactionEvent.GetErrorDescription())
	}
	// map the transaction type/id
	transactionID := transactionEvent.GetTransactionId()
	switch casted := transactionID.(type) {
	case *model_v1.SpanData_TransactionEvent_LocalId:
		clientEvent.Attributes().InsertInt(transactionIDEventKey, int64(casted.LocalId.TransactionId))
		clientEvent.Attributes().InsertString(transactedSessionNameEventKey, casted.LocalId.SessionName)
		clientEvent.Attributes().InsertInt(transactedSessionIDEventKey, int64(casted.LocalId.SessionId))
	case *model_v1.SpanData_TransactionEvent_Xid_:
		// format xxxxxxxx-yyyyyyyy-zzzzzzzz where x is FormatID (hex rep of int32), y is BranchQualifier and z is GlobalID, hex encoded.
		xidString := fmt.Sprintf("%08x", casted.Xid.FormatId) + "-" +
			hex.EncodeToString(casted.Xid.BranchQualifier) + "-" + hex.EncodeToString(casted.Xid.GlobalId)
		clientEvent.Attributes().InsertString(transactionXIDEventKey, xidString)
	default:
		u.logger.Warn(fmt.Sprintf("Unknown transaction ID type %T", transactionID))
		recordRecoverableUnmarshallingError()
	}
}

func (u *solaceMessageUnmarshallerV1) rgmidToString(rgmid []byte) string {
	// rgmid[0] is the version of the rgmid
	if len(rgmid) != 17 || rgmid[0] != 1 {
		// may be cases where the rgmid is empty or nil, len(rgmid) will return 0 if nil
		if len(rgmid) > 0 {
			u.logger.Warn("Received invalid length or version for rgmid", zap.Int8("version", int8(rgmid[0])), zap.Int("length", len(rgmid)))
			recordRecoverableUnmarshallingError()
		}
		return hex.EncodeToString(rgmid)
	}
	rgmidEncoded := make([]byte, 32)
	hex.Encode(rgmidEncoded, rgmid[1:])
	// format: rmid1:aaaaa-bbbbbbbbbbb-cccccccc-dddddddd
	rgmidString := "rmid1:" + string(rgmidEncoded[0:5]) + "-" + string(rgmidEncoded[5:16]) + "-" + string(rgmidEncoded[16:24]) + "-" + string(rgmidEncoded[24:32])
	return rgmidString
}

// insertUserProperty will instert a user property value with the given key to an attribute if possible.
// Since AttributeMap only supports int64 integer types, uint64 data may be misrepresented.
func (u solaceMessageUnmarshallerV1) insertUserProperty(toMap *pcommon.Map, key string, value interface{}) {
	const (
		// userPropertiesPrefixAttrKey is the key used to prefix all user properties
		userPropertiesAttrKeyPrefix = "messaging.solace.user_properties."
	)
	k := userPropertiesAttrKeyPrefix + key
	switch v := value.(type) {
	case *model_v1.SpanData_UserPropertyValue_NullValue:
		toMap.Insert(k, pcommon.NewValueEmpty())
	case *model_v1.SpanData_UserPropertyValue_BoolValue:
		toMap.InsertBool(k, v.BoolValue)
	case *model_v1.SpanData_UserPropertyValue_DoubleValue:
		toMap.InsertDouble(k, v.DoubleValue)
	case *model_v1.SpanData_UserPropertyValue_ByteArrayValue:
		toMap.InsertBytes(k, pcommon.NewImmutableByteSlice(v.ByteArrayValue))
	case *model_v1.SpanData_UserPropertyValue_FloatValue:
		toMap.InsertDouble(k, float64(v.FloatValue))
	case *model_v1.SpanData_UserPropertyValue_Int8Value:
		toMap.InsertInt(k, int64(v.Int8Value))
	case *model_v1.SpanData_UserPropertyValue_Int16Value:
		toMap.InsertInt(k, int64(v.Int16Value))
	case *model_v1.SpanData_UserPropertyValue_Int32Value:
		toMap.InsertInt(k, int64(v.Int32Value))
	case *model_v1.SpanData_UserPropertyValue_Int64Value:
		toMap.InsertInt(k, v.Int64Value)
	case *model_v1.SpanData_UserPropertyValue_Uint8Value:
		toMap.InsertInt(k, int64(v.Uint8Value))
	case *model_v1.SpanData_UserPropertyValue_Uint16Value:
		toMap.InsertInt(k, int64(v.Uint16Value))
	case *model_v1.SpanData_UserPropertyValue_Uint32Value:
		toMap.InsertInt(k, int64(v.Uint32Value))
	case *model_v1.SpanData_UserPropertyValue_Uint64Value:
		toMap.InsertInt(k, int64(v.Uint64Value))
	case *model_v1.SpanData_UserPropertyValue_StringValue:
		toMap.InsertString(k, v.StringValue)
	case *model_v1.SpanData_UserPropertyValue_DestinationValue:
		toMap.InsertString(k, v.DestinationValue)
	default:
		u.logger.Warn(fmt.Sprintf("Unknown user property type: %T", v))
		recordRecoverableUnmarshallingError()
	}
}
