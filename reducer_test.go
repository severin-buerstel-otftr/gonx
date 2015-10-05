package gonx

import (
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestReducer(t *testing.T) {
	Convey("Test process input channel with reducers", t, func() {
		input := make(chan *Entry, 10)

		Convey("ReadAll reducer", func() {
			reducer := new(ReadAll)

			// Prepare import channel
			entry := NewEmptyEntry()
			input <- entry
			close(input)

			output := make(chan *Entry, 1) // Make it buffered to avoid deadlock
			reducer.Reduce(input, output)

			// ReadAll reducer writes input channel to the output
			result, ok := <-output
			So(ok, ShouldBeTrue)
			So(result, ShouldEqual, entry)
		})

		Convey("With filled input channel", func() {
			// Prepare import channel
			input <- NewEntry(Fields{
				"uri": "/asd/fgh",
				"foo": "123",
				"bar": "234",
				"baz": "345",
			})
			input <- NewEntry(Fields{
				"uri": "/zxc/vbn",
				"foo": "456",
				"bar": "567",
				"baz": "678",
			})
			close(input)

			output := make(chan *Entry, 1) // Make it buffered to avoid deadlock

			Convey("Count reducer", func() {
				reducer := new(Count)
				reducer.Reduce(input, output)

				result, ok := <-output
				So(ok, ShouldBeTrue)
				count, err := result.Field("count")
				So(err, ShouldBeNil)
				So(count, ShouldEqual, "2")
			})

			Convey("Sum reducer", func() {
				reducer := &Sum{[]string{"foo", "bar"}}
				reducer.Reduce(input, output)

				result, ok := <-output
				So(ok, ShouldBeTrue)
				value, err := result.FloatField("foo")
				So(err, ShouldBeNil)
				So(value, ShouldEqual, 123.0+456)
				value, err = result.FloatField("bar")
				So(err, ShouldBeNil)
				So(value, ShouldEqual, 234.0+567)
				_, err = result.Field("buz")
				So(err, ShouldNotBeNil)
			})

			Convey("Avg reducer", func() {
				reducer := &Avg{[]string{"foo", "bar"}}
				reducer.Reduce(input, output)

				result, ok := <-output
				So(ok, ShouldBeTrue)
				value, err := result.FloatField("foo")
				So(err, ShouldBeNil)
				So(value, ShouldEqual, (123.0+456.0)/2.0)
				value, err = result.FloatField("bar")
				So(err, ShouldBeNil)
				So(value, ShouldEqual, (234.0+567.0)/2.0)
				_, err = result.Field("buz")
				So(err, ShouldNotBeNil)
			})
		})
	})
}

func TestChainReducer(t *testing.T) {
	reducer := NewChain(&Avg{[]string{"foo", "bar"}}, &Count{})
	assert.Implements(t, (*Reducer)(nil), reducer)

	// Prepare import channel
	input := make(chan *Entry, 2)
	input <- NewEntry(Fields{
		"uri": "/asd/fgh",
		"foo": "123",
		"bar": "234",
		"baz": "345",
	})
	input <- NewEntry(Fields{
		"uri": "/zxc/vbn",
		"foo": "456",
		"bar": "567",
		"baz": "678",
	})
	close(input)
	output := make(chan *Entry, 1) // Make it buffered to avoid deadlock
	reducer.Reduce(input, output)

	result, ok := <-output
	assert.True(t, ok)

	value, err := result.FloatField("foo")
	assert.NoError(t, err)
	assert.Equal(t, value, (123.0+456)/2.0)

	value, err = result.FloatField("bar")
	assert.NoError(t, err)
	assert.Equal(t, value, (234.0+567.0)/2.0)

	count, err := result.Field("count")
	assert.NoError(t, err)
	assert.Equal(t, count, "2")

	_, err = result.Field("buz")
	assert.Error(t, err)
}

func TestGroupByReducer(t *testing.T) {
	reducer := NewGroupBy(
		// Fields to group by
		[]string{"host"},
		// Result reducers
		&Sum{[]string{"foo", "bar"}},
		new(Count),
	)
	assert.Implements(t, (*Reducer)(nil), reducer)

	// Prepare import channel
	input := make(chan *Entry, 10)
	input <- NewEntry(Fields{
		"uri":  "/asd/fgh",
		"host": "alpha.example.com",
		"foo":  "1",
		"bar":  "2",
		"baz":  "3",
	})
	input <- NewEntry(Fields{
		"uri":  "/zxc/vbn",
		"host": "beta.example.com",
		"foo":  "4",
		"bar":  "5",
		"baz":  "6",
	})
	input <- NewEntry(Fields{
		"uri":  "/ijk/lmn",
		"host": "beta.example.com",
		"foo":  "7",
		"bar":  "8",
		"baz":  "9",
	})
	close(input)
	output := make(chan *Entry, 2) // Make it buffered to avoid deadlock
	reducer.Reduce(input, output)

	// Collect result entries from output channel to the map, because reading
	// from channel can be in any order, it depends on each reducer processing
	resultMap := make(map[string]*Entry)
	for result := range output {
		value, err := result.Field("host")
		assert.NoError(t, err)
		resultMap[value] = result
	}
	assert.Equal(t, len(resultMap), 2)

	// Read and assert first group result
	result := resultMap["alpha.example.com"]

	floatVal, err := result.FloatField("foo")
	assert.NoError(t, err)
	assert.Equal(t, floatVal, 1.0)

	floatVal, err = result.FloatField("bar")
	assert.NoError(t, err)
	assert.Equal(t, floatVal, 2.0)

	value, err := result.Field("count")
	assert.NoError(t, err)
	assert.Equal(t, value, "1")

	// Read and assert second group result
	result = resultMap["beta.example.com"]

	floatVal, err = result.FloatField("foo")
	assert.NoError(t, err)
	assert.Equal(t, floatVal, 4.0+7.0)

	floatVal, err = result.FloatField("bar")
	assert.NoError(t, err)
	assert.Equal(t, floatVal, 5.0+8.0)

	value, err = result.Field("count")
	assert.NoError(t, err)
	assert.Equal(t, value, "2")
}
