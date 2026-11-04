/*******************************************************************************
 * Copyright 2017.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 *******************************************************************************/

package messagestore

import "sync"

type MsgAckChan struct {
	Mu       sync.Mutex
	Id       string
	IsClosed bool
	DataChan chan interface{}
	// onClose 关闭时从 MessageStore 的 map 中移除自身，避免超时/失败调用永久堆积。
	onClose func()
}

func (mac *MsgAckChan) closeLocked() {
	if mac.IsClosed {
		return
	}
	close(mac.DataChan)
	mac.IsClosed = true
	if mac.onClose != nil {
		mac.onClose()
	}
}

func (mac *MsgAckChan) TryCloseChan() {
	mac.Mu.Lock()
	defer mac.Mu.Unlock()
	mac.closeLocked()
}

func (mac *MsgAckChan) TrySendDataAndCloseChan(data interface{}) bool {
	mac.Mu.Lock()
	defer mac.Mu.Unlock()
	if !mac.IsClosed {
		mac.DataChan <- data
		mac.closeLocked()
		return true
	}
	return false
}
