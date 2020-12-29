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
	lock			   sync.RWMutex
}

func NewTransparentCache(actualPriceService PriceService, maxAge time.Duration) *TransparentCache {
	return &TransparentCache{
		actualPriceService: actualPriceService,
		maxAge:             maxAge,
		prices:             map[string]ValueTimestampPair{},
		lock:				sync.RWMutex{},
	}
}

func (c *TransparentCache) doGetPriceFor(itemCode string) (ValueTimestampPair, bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	value, ok := c.prices[itemCode]
	return value, ok
}

func (c *TransparentCache) newPriceFor(itemCode string, itemPrice float64) (ValueTimestampPair) {
	c.lock.Lock()
	defer c.lock.Unlock()

	value, ok := c.prices[itemCode]

	if ok {
		return value
	}

	c.prices[itemCode] = NewValueTimestampPair(itemPrice, time.Now())
	return c.prices[itemCode]
}

func (c *TransparentCache) getPriceFor(itemCode string) (ValueTimestampPair, error) {
	value, ok := c.doGetPriceFor(itemCode)

	if ok {
		return value, nil
	}

	priceValue, err := c.actualPriceService.GetPriceFor(itemCode)
	if err != nil {
		return NewValueTimestampPair(0, time.Now()), fmt.Errorf("getting price from service : %v", err.Error())
	}

	return c.newPriceFor(itemCode, priceValue), nil
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

// Receives a reference to a cache and an item code.
// Updates the value stored in the cache for this item and its associated timestamp.
// Returns the new cached value for the item. If an error occurs, then the error is returned instead.
func (valueTimestamp *ValueTimestampPair) updateAndGetValue(c *TransparentCache, itemCode string) (float64, error){
	valueTimestamp.lock.Lock()
	defer valueTimestamp.lock.Unlock()

	if time.Now().Sub(valueTimestamp.getTimestamp()) < c.maxAge {
		return valueTimestamp.value, nil
	}

	newValue, err := c.actualPriceService.GetPriceFor(itemCode)
	valueTimestamp.value = newValue
	valueTimestamp.updateTimestamp(time.Now())

	return newValue, err
}

// Receives an expected maximum age. If the age associated to the ValueTimestampPair does not exceed
// the maximum age, then the value is returned. Otherwise a value of 0 is returned together with an error.
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
	price, err := c.getPriceFor(itemCode)
	if err == nil {
		priceValue, err := price.getValueIfTimestampNotExpired(c.maxAge)
		if err == nil {
			return priceValue, nil
		}
		//We need to refresh this price in the cache
		return price.updateAndGetValue(c, itemCode)
	}
	return 0, err
}

// Receives an item and a channel. Attempts to retrieve the item's price from the cache and send a Message containing
// the retrieved value through the channel. If an error occurred while fetching the item's price, the Message will contain
// the error instead. 
func (c *TransparentCache) getPriceForItemAndSendThroughChannel(itemCode string, resultChannel chan Message) {
	result, err := c.GetPriceFor(itemCode)
	resultChannel <- Message {
		value:	result, 
		err: 	err,
	}
	close(resultChannel)
}

// GetPricesFor gets the prices for several items at once, some might be found in the cache, others might not
// If any of the operations returns an error, it should return an error as well
func (c *TransparentCache) GetPricesFor(itemCodes ...string) ([]float64, error) {
	channels := []chan Message{}

	for _, itemCode := range itemCodes {
		var channel = make(chan Message)
		go c.getPriceForItemAndSendThroughChannel(itemCode, channel)
		channels = append(channels, channel)
	}

	results := make([]float64, len(itemCodes))

	// Iterate through channels to retrieve results in order
	for i, _ := range itemCodes {
		message, ok := <- channels[i]
		if !ok {
			return []float64{}, message.err
		}
		results[i] = message.value
	}
	return results, nil
}
