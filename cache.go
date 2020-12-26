package sample1

import (
	"time"
	"fmt"
	"sync"
)

type Message struct {
	value float64
	err error
}

// PriceService is a service that we can use to get prices for the items
// Calls to this service are expensive (they take time)
type PriceService interface {
	GetPriceFor(itemCode string) (float64, error)
}

// TransparentCache is a cache that wraps the actual service
// The cache will remember prices we ask for, so that we don't have to wait on every call
// Cache should only return a price if it is not older than "maxAge", so that we don't get stale prices
type TransparentCache struct {
	actualPriceService PriceService
	maxAge             time.Duration
	prices             map[string]ValueTimestampPair
	lock			   sync.Mutex
}

func NewTransparentCache(actualPriceService PriceService, maxAge time.Duration) *TransparentCache {
	return &TransparentCache{
		actualPriceService: actualPriceService,
		maxAge:             maxAge,
		prices:             map[string]ValueTimestampPair{},
		lock:				sync.Mutex{},
	}
}

func (c *TransparentCache) getPriceFor(itemCode string) (float64, error) {
	priceValue, err := c.actualPriceService.GetPriceFor(itemCode)
	if err != nil {
		return 0, fmt.Errorf("getting price from service : %v", err.Error())
	}
	c.lock.Lock()
	_, ok := c.prices[itemCode]
	if !ok {
		c.prices[itemCode] = NewValueTimestampPair(priceValue, time.Now())
	}
	c.lock.Unlock()

	return priceValue, nil
}

type Timestamp struct {
	timestamp	time.Time
}

type ValueTimestampPair struct {
	value		float64
	timestamp	*Timestamp
	lock		*sync.RWMutex
}

func NewValueTimestampPair(value float64, time time.Time) ValueTimestampPair {
	return ValueTimestampPair{
		value:		value,
		timestamp: 	&Timestamp{timestamp:time},
		lock:		&sync.RWMutex{},
	}
}

func (valueTimestamp *ValueTimestampPair) getTimestamp() (time.Time) {
	return valueTimestamp.timestamp.timestamp
}

func (valueTimestamp *ValueTimestampPair) updateTimestamp(time time.Time) {
	valueTimestamp.timestamp.timestamp = time
}

func (valueTimestamp *ValueTimestampPair) updateAndGetValue(c *TransparentCache, itemCode string) (float64, error){
	valueTimestamp.lock.Lock()
	defer valueTimestamp.lock.Unlock()
	//fmt.Printf("ENTER MISS: key: %s \n", itemCode)


	if time.Now().Sub(valueTimestamp.getTimestamp()) < c.maxAge {
		//fmt.Printf("MISS BUT THEN HIT: key: %s\n", itemCode)

		return valueTimestamp.value, nil
	}

	newValue, err := c.actualPriceService.GetPriceFor(itemCode)
	valueTimestamp.value = newValue
	valueTimestamp.updateTimestamp(time.Now())
	//fmt.Printf("MISS: key: %s\n", itemCode)

	return newValue, err
}

func (valueTimestamp *ValueTimestampPair) getValueIfTimestampNotExpired(maxAge time.Duration) (float64, error) {
	valueTimestamp.lock.RLock()
	defer valueTimestamp.lock.RUnlock()

	// Check that the price was retrieved less than "maxAge" ago!
	if time.Now().Sub(valueTimestamp.getTimestamp()) < maxAge {
		//The price is still valid, so we return the cached value
		return valueTimestamp.value, nil
	}

	return 0, fmt.Errorf("Attempted to obtain an expired value from cache")
}

// GetPriceFor gets the price for the item, either from the cache or the actual service if it was not cached or too old
func (c *TransparentCache) GetPriceFor(itemCode string) (float64, error) {
	price, ok := c.prices[itemCode]
	if ok {
		priceValue, err := price.getValueIfTimestampNotExpired(c.maxAge)
		if err == nil {
			return priceValue, nil
		}
		//We need to refresh this price in the cache
		return price.updateAndGetValue(c, itemCode)
	}
	//The item was never loaded to the cache
	return c.getPriceFor(itemCode)
}

func (c *TransparentCache) GetPriceForItemAndSendThroughChannel(itemCode string, resultChannel chan Message) {
	result, err := c.GetPriceFor(itemCode)
	resultChannel <- Message {
		value:	result, 
		err: 	err,
	}
}

// GetPricesFor gets the prices for several items at once, some might be found in the cache, others might not
// If any of the operations returns an error, it should return an error as well
func (c *TransparentCache) GetPricesFor(itemCodes ...string) ([]float64, error) {
	results := []float64{}
	channel := make(chan Message)
	for _, itemCode := range itemCodes {
		//fmt.Printf("Process item %s\n", itemCode)
		go c.GetPriceForItemAndSendThroughChannel(itemCode, channel)
	}
	for range itemCodes {
		message, ok := <-channel
		if message.err != nil {
			return []float64{}, message.err
		}
		if ok {
			//fmt.Printf("Append result for item %s\n", itemCode)
			results = append(results, message.value)
		}
	}
	return results, nil
}
