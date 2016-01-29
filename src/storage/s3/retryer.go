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
	"math"
	"math/rand"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/request"
)

// Retryer is a custom aws retrier that logs retry attempts
type Retryer struct {
	client.DefaultRetryer

	retryDelay int
	retryMax   int
}

// NewRetryer returns the instance of Retryer object
func NewRetryer(retryDelay, retryMax int) *Retryer {
	return &Retryer{
		retryDelay: retryDelay,
		retryMax:   retryMax,
	}
}

// MaxRetries returns the number of maximum returns the service will use to make
// an individual API request.
func (d Retryer) MaxRetries() int {
	return d.retryMax
}

// RetryRules returns the delay duration before retrying this request again
func (d Retryer) RetryRules(r *request.Request) time.Duration {
	duration := d.getDuratoin(r.RetryCount)

	log.Errorf("%s/%s failed, will retry in %s, error %v",
		r.ClientInfo.ServiceName, r.Operation.Name, duration, r.Error)

	return duration
}

// Outer is for external stuff to handle retries for cases that are
// not covered by s3manager https://github.com/aws/aws-sdk-go/issues/466
func (d Retryer) Outer(f func() error) error {
	n := 0

	for {
		if err := f(); err != nil {
			if n == d.retryMax {
				log.Errorf("Max retries %d reached, error: %s", d.retryMax, err)
			}
			if _, ok := err.(awserr.Error); ok || n == d.retryMax {
				return err
			}

			duration := d.getDuratoin(n)
			n = n + 1

			log.Errorf("Retry %d/%d after %s, error: %s", n, d.retryMax, duration, err)
			time.Sleep(duration)

			continue
		}

		break
	}

	return nil
}

func (d Retryer) getDuratoin(n int) time.Duration {
	delay := int(math.Pow(2, float64(n))) * (rand.Intn(d.retryDelay) + d.retryDelay)
	return time.Duration(delay) * time.Millisecond
}
