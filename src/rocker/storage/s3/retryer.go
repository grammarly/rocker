/*-
 * Copyright 2015 Grammarly, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package s3

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/request"
)

var (
	retryDelay = 400
	retryMax   = 6
)

// Retryer is a custom aws retrier that logs retry attempts
type Retryer struct {
	client.DefaultRetryer
}

// MaxRetries returns the number of maximum returns the service will use to make
// an individual API request.
func (d Retryer) MaxRetries() int {
	return retryMax
}

// RetryRules returns the delay duration before retrying this request again
func (d Retryer) RetryRules(r *request.Request) time.Duration {
	delay := int(math.Pow(2, float64(r.RetryCount))) * (rand.Intn(retryDelay) + retryDelay)
	duration := time.Duration(delay) * time.Millisecond

	log.Errorf("%s/%s failed, will retry in %s, error %v",
		r.ClientInfo.ServiceName, r.Operation.Name, duration, r.Error)

	return duration
}

// globalRetry is for external stuff to handle retries for cases that are
// not covered by s3manager https://github.com/aws/aws-sdk-go/issues/466
func globalRetry(f func() error) error {
	n := 0

	for {
		if err := f(); err != nil {
			if _, ok := err.(awserr.Error); ok {
				return err
			}
			if n == retryMax {
				return fmt.Errorf("Max retries %d reached, error: %s", retryMax, err)
			}

			delay := int(math.Pow(2, float64(n))) * (rand.Intn(retryDelay) + retryDelay)
			duration := time.Duration(delay) * time.Millisecond
			n = n + 1

			log.Errorf("Retry %d/%d after %s, error: %s", n, retryMax, duration, err)
			time.Sleep(duration)

			continue
		}

		break
	}

	return nil
}
