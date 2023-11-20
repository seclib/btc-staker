package babylonclient

import (
	"errors"
	"fmt"
	"sync"

	"github.com/babylonchain/btc-staker/utils"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/sirupsen/logrus"
)

var (
	ErrBabylonBtcLightClientNotReady = errors.New("babylon btc light client is not ready to receive delegation")
)

type sendDelegationRequest struct {
	utils.Request[*sdk.TxResponse]
	dg                          *DelegationData
	requiredInclusionBlockDepth uint64
}

func newSendDelegationRequest(
	dg *DelegationData,
	requiredInclusionBlockDepth uint64,
) sendDelegationRequest {
	return sendDelegationRequest{
		Request:                     utils.NewRequest[*sdk.TxResponse](),
		dg:                          dg,
		requiredInclusionBlockDepth: requiredInclusionBlockDepth,
	}
}

type sendUndelegationRequest struct {
	utils.Request[*sdk.TxResponse]
	stakingTxHash *chainhash.Hash
	ud            *UndelegationData
}

func newSendUndelegationRequest(
	ud *UndelegationData,
	stakingTxHash *chainhash.Hash,
) sendUndelegationRequest {
	return sendUndelegationRequest{
		Request:       utils.NewRequest[*sdk.TxResponse](),
		ud:            ud,
		stakingTxHash: stakingTxHash,
	}
}

// BabylonMsgSender is responsible for sending delegation and undelegation requests to babylon
// It makes sure:
// - that babylon is ready for either delgetion or undelegation
// - only one messegae is sent to babylon at a time
type BabylonMsgSender struct {
	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup
	quit      chan struct{}

	cl                          BabylonClient
	logger                      *logrus.Logger
	sendDelegationRequestChan   chan *sendDelegationRequest
	sendUndelegationRequestChan chan *sendUndelegationRequest
}

func NewBabylonMsgSender(
	cl BabylonClient,
	logger *logrus.Logger,
) *BabylonMsgSender {
	return &BabylonMsgSender{
		quit:                        make(chan struct{}),
		cl:                          cl,
		logger:                      logger,
		sendDelegationRequestChan:   make(chan *sendDelegationRequest),
		sendUndelegationRequestChan: make(chan *sendUndelegationRequest),
	}
}

func (b *BabylonMsgSender) Start() {
	b.startOnce.Do(func() {
		b.wg.Add(1)
		go b.handleSentToBabylon()
	})
}

func (b *BabylonMsgSender) Stop() {
	b.stopOnce.Do(func() {
		close(b.quit)
		b.wg.Wait()
	})
}

// isBabylonBtcLcReady checks if Babylon BTC light client is ready to receive delegation
func (b *BabylonMsgSender) isBabylonBtcLcReady(
	requiredInclusionBlockDepth uint64,
	req *DelegationData,
) error {
	depth, err := b.cl.QueryHeaderDepth(req.StakingTransactionInclusionBlockHash)

	if err != nil {
		// If header is not known to babylon, or it is on LCFork, then most probably
		// lc is not up to date. We should retry sending delegation after some time.
		if errors.Is(err, ErrHeaderNotKnownToBabylon) || errors.Is(err, ErrHeaderOnBabylonLCFork) {
			return fmt.Errorf("btc light client error %s: %w", err.Error(), ErrBabylonBtcLightClientNotReady)
		}

		// got some unknown error, return it to the caller
		return fmt.Errorf("error while getting delegation data: %w", err)
	}

	if depth < requiredInclusionBlockDepth {
		return fmt.Errorf("btc lc not ready, required depth: %d, current depth: %d: %w", requiredInclusionBlockDepth, depth, ErrBabylonBtcLightClientNotReady)
	}

	return nil
}

func (m *BabylonMsgSender) handleSentToBabylon() {
	defer m.wg.Done()
	for {
		select {
		case req := <-m.sendDelegationRequestChan:
			stakingTxHash := req.dg.StakingTransaction.TxHash()

			err := m.isBabylonBtcLcReady(
				req.requiredInclusionBlockDepth,
				req.dg,
			)

			if err != nil {
				m.logger.WithFields(logrus.Fields{
					"btcTxHash": stakingTxHash,
					"err":       err,
				}).Error("Cannot send delegation request to babylon")

				req.ErrorChan() <- err
				continue
			}

			txResp, err := m.cl.Delegate(req.dg)

			if err != nil {
				if errors.Is(err, ErrInvalidBabylonExecution) {
					m.logger.WithFields(logrus.Fields{
						"btcTxHash":          stakingTxHash,
						"babylonTxHash":      txResp.TxHash,
						"babylonBlockHeight": txResp.Height,
						"babylonErrorCode":   txResp.Code,
						"babylonLog":         txResp.RawLog,
					}).Error("Invalid delegation data sent to babylon")
				}

				m.logger.WithFields(logrus.Fields{
					"btcTxHash": stakingTxHash,
					"err":       err,
				}).Error("Error while sending delegation data to babylon")

				req.ErrorChan() <- fmt.Errorf("failed to send delegation for tx with hash: %s: %w", stakingTxHash.String(), err)
			}

			req.ResultChan() <- txResp

		case req := <-m.sendUndelegationRequestChan:
			di, err := m.cl.QueryDelegationInfo(req.stakingTxHash)

			if err != nil {
				req.ErrorChan() <- fmt.Errorf("failed to retrieve delegation info for staking tx with hash: %s: %w", req.stakingTxHash.String(), err)
				continue
			}

			if !di.Active {
				req.ErrorChan() <- fmt.Errorf("cannot sent unbonding request for staking tx with hash: %s, as delegation is not active", req.stakingTxHash.String())
				continue
			}

			if di.UndelegationInfo != nil {
				req.ErrorChan() <- fmt.Errorf("cannot sent unbonding request for staking tx with hash: %s, as unbonding request was already sent", req.stakingTxHash.String())
				continue
			}

			txResp, err := m.cl.Undelegate(req.ud)

			if err != nil {
				if errors.Is(err, ErrInvalidBabylonExecution) {
					// Additional logging if for some reason we send unbonding request which was
					// accepted by babylon, but failed execution
					m.logger.WithFields(logrus.Fields{
						"btcTxHash":          req.stakingTxHash.String(),
						"babylonTxHash":      txResp.TxHash,
						"babylonBlockHeight": txResp.Height,
						"babylonErrorCode":   txResp.Code,
						"babylonLog":         txResp.RawLog,
					}).Error("Invalid delegation data sent to babylon")
				}

				m.logger.WithFields(logrus.Fields{
					"btcTxHash": req.stakingTxHash,
					"err":       err,
				}).Error("Error while sending undelegation data to babylon")

				req.ErrorChan() <- fmt.Errorf("failed to send unbonding for delegation with staking hash:%s:%w", req.stakingTxHash.String(), err)
				continue
			}

			req.ResultChan() <- txResp

		case <-m.quit:
			return
		}
	}
}

func (m *BabylonMsgSender) SendDelegation(
	dg *DelegationData,
	requiredInclusionBlockDepth uint64,
) (*sdk.TxResponse, error) {
	req := newSendDelegationRequest(dg, requiredInclusionBlockDepth)

	return utils.SendRequestAndWaitForResponseOrQuit[*sdk.TxResponse, *sendDelegationRequest](
		&req,
		m.sendDelegationRequestChan,
		m.quit,
	)

}

func (m *BabylonMsgSender) SendUndelegation(
	ud *UndelegationData,
	stakingTxHash *chainhash.Hash,
) (*sdk.TxResponse, error) {
	req := newSendUndelegationRequest(ud, stakingTxHash)

	return utils.SendRequestAndWaitForResponseOrQuit[*sdk.TxResponse, *sendUndelegationRequest](
		&req,
		m.sendUndelegationRequestChan,
		m.quit,
	)
}