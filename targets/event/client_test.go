package event

import (
	"context"
	"fmt"
	"github.com/kubemq-hub/kubemq-sources/config"
	"github.com/kubemq-hub/kubemq-sources/types"
	"github.com/kubemq-io/kubemq-go"

	"github.com/stretchr/testify/require"
	"testing"

	"time"
)

type mockEventReceiver struct {
	host    string
	port    int
	channel string
	timeout time.Duration
}

func (m *mockEventReceiver) run(ctx context.Context) (*types.Request, error) {
	client, err := kubemq.NewClient(ctx,
		kubemq.WithAddress(m.host, m.port),
		kubemq.WithClientId("response-id"),
		kubemq.WithTransportType(kubemq.TransportTypeGRPC),
		kubemq.WithCheckConnection(true))
	if err != nil {
		return nil, err
	}
	errCh := make(chan error, 1)
	eventCh, err := client.SubscribeToEvents(ctx, m.channel, "", errCh)
	if err != nil {
		return nil, err
	}
	select {
	case event := <-eventCh:
		if event == nil {
			return nil, nil
		}
		return types.ParseRequestFromEvent(event)
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, nil

	case <-time.After(m.timeout):
		return nil, fmt.Errorf("timeout")
	}

}

func TestClient_Do(t *testing.T) {
	tests := []struct {
		name         string
		cfg          config.Metadata
		mockReceiver *mockEventReceiver
		sendReq      *types.Request
		wantReq      *types.Request
		wantResp     *types.Response
		wantErr      bool
	}{
		{
			name: "request",
			cfg: config.Metadata{
				Name: "kubemq-target",
				Kind: "",
				Properties: map[string]string{
					"host": "localhost",
					"port": "50000",
				},
			},
			mockReceiver: &mockEventReceiver{
				host:    "localhost",
				port:    50000,
				channel: "events",
				timeout: 5 * time.Second,
			},
			sendReq: types.NewRequest().
				SetData([]byte("data")).
				SetMetadataKeyValue("id", "id").
				SetMetadataKeyValue("channel", "events"),
			wantReq: types.NewRequest().
				SetData([]byte("data")),
			wantResp: types.NewResponse().
				SetMetadataKeyValue("error", "").
				SetMetadataKeyValue("event_id", "id"),
			wantErr: false,
		},
		{
			name: "request error - no data",
			cfg: config.Metadata{
				Name: "kubemq-target",
				Kind: "",
				Properties: map[string]string{
					"host": "localhost",
					"port": "50000",
				},
			},
			mockReceiver: &mockEventReceiver{
				host:    "localhost",
				port:    50000,
				channel: "events",
				timeout: 5 * time.Second,
			},
			sendReq: types.NewRequest().
				SetMetadataKeyValue("id", "id").
				SetMetadataKeyValue("channel", "events"),
			wantReq:  nil,
			wantResp: nil,
			wantErr:  true,
		},
		{
			name: "request error - bad metadata - empty channel",
			cfg: config.Metadata{
				Name: "kubemq-target",
				Kind: "",
				Properties: map[string]string{
					"host": "localhost",
					"port": "50000",
				},
			},
			mockReceiver: &mockEventReceiver{
				host:    "localhost",
				port:    50000,
				channel: "events",
				timeout: 5 * time.Second,
			},
			sendReq: types.NewRequest().
				SetData([]byte("data")).
				SetMetadataKeyValue("id", "id").
				SetMetadataKeyValue("channel", ""),
			wantReq:  nil,
			wantResp: nil,
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			recRequestCh := make(chan *types.Request, 1)
			recErrCh := make(chan error, 1)
			go func() {
				gotRequest, err := tt.mockReceiver.run(ctx)
				select {
				case recErrCh <- err:
				case recRequestCh <- gotRequest:
				}
			}()
			time.Sleep(time.Second)
			target := New()
			err := target.Init(ctx, tt.cfg)
			require.NoError(t, err)
			gotResp, err := target.Do(ctx, tt.sendReq)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.EqualValues(t, tt.wantResp, gotResp)
			select {
			case gotRequest := <-recRequestCh:
				require.EqualValues(t, tt.wantReq, gotRequest)
			case err := <-recErrCh:
				require.NoError(t, err)
			case <-ctx.Done():
				require.NoError(t, ctx.Err())
			}
		})
	}
}

func TestClient_Init(t *testing.T) {

	tests := []struct {
		name    string
		cfg     config.Metadata
		wantErr bool
	}{
		{
			name: "init",
			cfg: config.Metadata{
				Name: "kubemq-target",
				Kind: "",
				Properties: map[string]string{
					"host":            "localhost",
					"port":            "50000",
					"client_id":       "client_id",
					"auth_token":      "some-auth token",
					"default_channel": "some-channel",
				},
			},
			wantErr: false,
		},
		{
			name: "init - error",
			cfg: config.Metadata{
				Name: "kubemq-target",
				Kind: "",
				Properties: map[string]string{
					"host": "localhost",
					"port": "-1",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			c := New()

			if err := c.Init(ctx, tt.cfg); (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.EqualValues(t, tt.cfg.Name, c.Name())
		})
	}
}
