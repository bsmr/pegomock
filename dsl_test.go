// Copyright 2015 Peter Goetz
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pegomock_test

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"

	. "github.com/petergtz/pegomock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/petergtz/pegomock/pegomock/testutil"
)

func AnyError() error {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf((*error)(nil)).Elem()))
	return nil
}

func AnyRequest() http.Request {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf(http.Request{})))
	return http.Request{}
}

func AnyRequestPtr() *http.Request {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf((*http.Request)(nil))))
	return nil
}

var _ = Describe("MockDisplay", func() {
	var display *MockDisplay

	BeforeEach(func() {
		display = NewMockDisplay()
	})

	Context("Calling SomeValue() with no stubbing", func() {
		It("returns zero value", func() {
			Expect(display.SomeValue()).To(Equal(""))
		})
	})

	Context("Stubbing MultipleParamsAndReturnValue() with matchers", func() {
		BeforeEach(func() {
			When(display.MultipleParamsAndReturnValue(EqString("Hello"), EqInt(333))).ThenReturn("Bla")
		})

		It("fails during verification when mock was not called", func() {
			Expect(func() { display.VerifyWasCalledOnce().MultipleParamsAndReturnValue("Hello", 333) }).To(PanicWith(
				"Mock invocation count for method \"MultipleParamsAndReturnValue\" with params [Hello 333] " +
					"does not match expectation.\n\n\tExpected: 1; but got: 0",
			))
		})

		It("succeeds verification when mock was called", func() {
			display.MultipleParamsAndReturnValue("Hello", 333)
			Expect(func() { display.VerifyWasCalledOnce().MultipleParamsAndReturnValue("Hello", 333) }).NotTo(Panic())
		})

		It("succeeds verification when verification and invocation are mixed", func() {
			Expect(func() { display.VerifyWasCalledOnce().MultipleParamsAndReturnValue("Hello", 333) }).To(PanicWith(
				"Mock invocation count for method \"MultipleParamsAndReturnValue\" with params [Hello 333] " +
					"does not match expectation.\n\n\tExpected: 1; but got: 0",
			))
			display.MultipleParamsAndReturnValue("Hello", 333)
			Expect(func() { display.VerifyWasCalledOnce().MultipleParamsAndReturnValue("Hello", 333) }).NotTo(Panic())
		})
	})

	Context("Calling MultipleParamsAndReturnValue() with \"Any\"-matchers", func() {
		It("succeeds all verifications that match", func() {
			When(display.MultipleParamsAndReturnValue(AnyString(), EqInt(333))).ThenReturn("Bla")

			Expect(func() { display.VerifyWasCalledOnce().MultipleParamsAndReturnValue("Hello", 333) }).To(PanicWith(
				"Mock invocation count for method \"MultipleParamsAndReturnValue\" with params [Hello 333] " +
					"does not match expectation.\n\n\tExpected: 1; but got: 0",
			))

			display.MultipleParamsAndReturnValue("Hello", 333)
			display.MultipleParamsAndReturnValue("Hello again", 333)
			display.MultipleParamsAndReturnValue("And again", 333)

			Expect(func() { display.VerifyWasCalledOnce().MultipleParamsAndReturnValue("Hello", 333) }).NotTo(Panic())
			Expect(func() { display.VerifyWasCalledOnce().MultipleParamsAndReturnValue("Hello again", 333) }).NotTo(Panic())
			Expect(func() { display.VerifyWasCalledOnce().MultipleParamsAndReturnValue("And again", 333) }).NotTo(Panic())

			Expect(func() { display.VerifyWasCalledOnce().MultipleParamsAndReturnValue("And again", 444) }).To(PanicWith(
				"Mock invocation count for method \"MultipleParamsAndReturnValue\" with params [And again 444] " +
					"does not match expectation.\n\n\tExpected: 1; but got: 0",
			))

		})
	})

	Context("Calling MultipleParamsAndReturnValue() only with matchers on some parameters", func() {
		It("panics", func() {
			Expect(func() { When(display.MultipleParamsAndReturnValue(EqString("Hello"), 333)) }).To(PanicWith(
				"Invalid use of matchers!\n\n 2 matchers expected, 1 recorded.\n\n" +
					"This error may occur if matchers are combined with raw values:\n" +
					"    //incorrect:\n" +
					"    someFunc(AnyInt(), \"raw String\")\n" +
					"When using matchers, all arguments have to be provided by matchers.\n" +
					"For example:\n" +
					"    //correct:\n" +
					"    someFunc(AnyInt(), EqString(\"String by matcher\"))",
			))
		})
	})

	Context("Stubbing with consecutive return values", func() {
		BeforeEach(func() {
			When(display.SomeValue()).ThenReturn("Hello").ThenReturn("again")
		})

		It("returns stubbed values when calling mock", func() {
			Expect(display.SomeValue()).To(Equal("Hello"))
			Expect(display.SomeValue()).To(Equal("again"))
		})

		It("returns last stubbed value repeatedly", func() {
			Expect(display.SomeValue()).To(Equal("Hello"))
			Expect(display.SomeValue()).To(Equal("again"))
			Expect(display.SomeValue()).To(Equal("again"))
			Expect(display.SomeValue()).To(Equal("again"))
			Expect(display.SomeValue()).To(Equal("again"))
			Expect(display.SomeValue()).To(Equal("again"))
		})

		It("can be verified that mock was called", func() {
			display.SomeValue()
			Expect(func() { display.VerifyWasCalledOnce().SomeValue() }).NotTo(Panic())
		})

		It("fails if verify is called on mock that was not invoked.", func() {
			Expect(func() { display.VerifyWasCalledOnce().Show("Some parameter") }).To(PanicWith(
				"Mock invocation count for method \"Show\" with params [Some parameter] " +
					"does not match expectation.\n\n\tExpected: 1; but got: 0",
			))
		})

		It("fails if verify is called on mock that was invoked more than once.", func() {
			display.Show("param")
			display.Show("param")
			Expect(func() { display.VerifyWasCalledOnce().Show("param") }).To(PanicWith(
				"Mock invocation count for method \"Show\" with params [param] " +
					"does not match expectation.\n\n\tExpected: 1; but got: 2",
			))

		})
	})

	Context("Stubbing with invalid return type", func() {
		It("panics", func() {
			Expect(func() { When(display.SomeValue()).ThenReturn("Hello").ThenReturn(0) }).To(PanicWith(
				"Return value of type int not assignable to return type string",
			))
		})
	})

	Describe("https://github.com/petergtz/pegomock/issues/24", func() {
		Context("Stubbing with nil value", func() {
			It("does not panic when return type is interface{}", func() {
				When(display.InterfaceReturnValue()).ThenReturn(nil)
				Expect(display.InterfaceReturnValue()).To(BeNil())
			})

			It("does not panic when return type is error interface", func() {
				When(display.ErrorReturnValue()).ThenReturn(nil)
				Expect(display.ErrorReturnValue()).To(BeNil())
			})
		})

		Context("Stubbing with value that implements interface{}", func() {
			It("does not panic", func() {
				When(display.InterfaceReturnValue()).ThenReturn("Hello")
				Expect(display.InterfaceReturnValue()).To(Equal("Hello"))
			})
		})

		Context("Stubbing with value that implements error interface", func() {
			It("does not panic", func() {
				When(display.ErrorReturnValue()).ThenReturn(errors.New("Ouch"))
				Expect(display.ErrorReturnValue()).To(Equal(errors.New("Ouch")))
			})
		})

		Context("Stubbing with value that does not implement error interface", func() {
			It("panics", func() {
				Expect(func() { When(display.ErrorReturnValue()).ThenReturn("Blub") }).To(PanicWith(
					"Return value of type string not assignable to return type error",
				))
			})
		})

		Context("Stubbing string return type with nil value", func() {
			It("panics", func() {
				Expect(func() { When(display.SomeValue()).ThenReturn(nil) }).To(PanicWith(
					"Return value 'nil' not assignable to return type string",
				))
			})
		})

	})

	Context("Stubbed method, but no invocation takes place", func() {
		It("fails during verification", func() {
			When(display.SomeValue()).ThenReturn("Hello")
			Expect(func() { display.VerifyWasCalledOnce().SomeValue() }).To(PanicWith(
				"Mock invocation count for method \"SomeValue\" with params [] does not match expectation.\n\n\tExpected: 1; but got: 0",
			))
		})
	})

	Context("Calling Flash() with specific arguments", func() {

		BeforeEach(func() { display.Flash("Hello", 333) })

		It("succeeds verification if values are matching", func() {
			Expect(func() { display.VerifyWasCalledOnce().Flash("Hello", 333) }).NotTo(Panic())
		})

		It("fails during verification if values are not matching", func() {
			Expect(func() { display.VerifyWasCalledOnce().Flash("Hello", 666) }).To(PanicWith(
				"Mock invocation count for method \"Flash\" with params [Hello 666] " +
					"does not match expectation.\n\n\tExpected: 1; but got: 0",
			))
		})

		It("succeeds during verification when using Any-matchers ", func() {
			Expect(func() { display.VerifyWasCalledOnce().Flash(AnyString(), AnyInt()) }).NotTo(Panic())
		})

		It("succeeds during verification when using valid Eq-matchers ", func() {
			Expect(func() { display.VerifyWasCalledOnce().Flash(EqString("Hello"), EqInt(333)) }).NotTo(Panic())
		})

		It("fails during verification when using invalid Eq-matchers ", func() {
			Expect(func() { display.VerifyWasCalledOnce().Flash(EqString("Invalid"), EqInt(-1)) }).To(PanicWith(
				"Mock invocation count for method \"Flash\" with params [Eq(Invalid) Eq(-1)] " +
					"does not match expectation.\n\n\tExpected: 1; but got: 0",
			))
		})

		It("fails when not using matchers for all params", func() {
			Expect(func() { display.VerifyWasCalledOnce().Flash("Hello", AnyInt()) }).To(PanicWith(
				"Invalid use of matchers!\n\n 2 matchers expected, 1 recorded.\n\n" +
					"This error may occur if matchers are combined with raw values:\n" +
					"    //incorrect:\n" +
					"    someFunc(AnyInt(), \"raw String\")\n" +
					"When using matchers, all arguments have to be provided by matchers.\n" +
					"For example:\n" +
					"    //correct:\n" +
					"    someFunc(AnyInt(), EqString(\"String by matcher\"))",
			))
		})
	})

	Context("Calling Flash() twice", func() {

		BeforeEach(func() {
			display.Flash("Hello", 333)
			display.Flash("Hello", 333)
		})

		It("succeeds verification if verifying with Times(2)", func() {
			Expect(func() { display.VerifyWasCalled(Times(2)).Flash("Hello", 333) }).NotTo(Panic())
		})

		It("fails during verification if verifying with VerifyWasCalledOnce", func() {
			Expect(func() { display.VerifyWasCalledOnce().Flash("Hello", 333) }).To(PanicWith(
				"Mock invocation count for method \"Flash\" with params [Hello 333] " +
					"does not match expectation.\n\n\tExpected: 1; but got: 2",
			))
		})

		It("fails during verification if verifying with Times(1)", func() {
			Expect(func() { display.VerifyWasCalled(Times(1)).Flash("Hello", 333) }).To(PanicWith(
				"Mock invocation count for method \"Flash\" with params [Hello 333] " +
					"does not match expectation.\n\n\tExpected: 1; but got: 2",
			))
		})

		It("succeeds during verification when using AtLeast(1)", func() {
			Expect(func() { display.VerifyWasCalled(AtLeast(1)).Flash("Hello", 333) }).NotTo(Panic())
		})

		It("succeeds during verification when using AtLeast(2)", func() {
			Expect(func() { display.VerifyWasCalled(AtLeast(2)).Flash("Hello", 333) }).NotTo(Panic())
		})

		It("fails during verification when using AtLeast(3)", func() {
			Expect(func() { display.VerifyWasCalled(AtLeast(3)).Flash("Hello", 333) }).To(PanicWith(
				"Mock invocation count for method \"Flash\" with params [Hello 333] " +
					"does not match expectation.\n\n\tExpected: at least 3; but got: 2",
			))
		})

		It("succeeds during verification when using Never()", func() {
			Expect(func() { display.VerifyWasCalled(Never()).Flash("Other value", 333) }).NotTo(Panic())
		})

		It("fails during verification when using Never()", func() {
			Expect(func() { display.VerifyWasCalled(Never()).Flash("Hello", 333) }).To(PanicWith(
				"Mock invocation count for method \"Flash\" with params [Hello 333] " +
					"does not match expectation.\n\n\tExpected: 0; but got: 2",
			))
		})

	})

	Context("Calling MultipleParamsAndReturnValue()", func() {

		It("panics when stubbed to panic", func() {
			When(display.MultipleParamsAndReturnValue(AnyString(), AnyInt())).
				ThenPanic("I'm panicking")
			Expect(func() {
				display.MultipleParamsAndReturnValue("Some string", 123)
			}).To(PanicWith("I'm panicking"))
		})

		It("calls back when stubbed to call back", func() {
			When(display.MultipleParamsAndReturnValue(AnyString(), AnyInt())).Then(
				func(params []Param) ReturnValues {
					return []ReturnValue{fmt.Sprintf("%v%v", params[0], params[1])}
				},
			)
			Expect(display.MultipleParamsAndReturnValue("string and ", 123)).
				To(Equal("string and 123"))
		})

	})

	Context("Making calls in a specific order", func() {

		BeforeEach(func() {
			display.Flash("Hello", 111)
			display.Flash("again", 222)
			display.Flash("and again", 333)
		})

		It("succeeds during InOrder verification when order is correct", func() {
			Expect(func() {
				inOrderContext := new(InOrderContext)
				display.VerifyWasCalledInOrder(Once(), inOrderContext).Flash("Hello", 111)
				display.VerifyWasCalledInOrder(Once(), inOrderContext).Flash("again", 222)
				display.VerifyWasCalledInOrder(Once(), inOrderContext).Flash("and again", 333)
			}).NotTo(PanicButReport())
		})

		It("succeeds during InOrder verification when order is correct, but not all invocations are verified", func() {
			Expect(func() {
				inOrder := new(InOrderContext)
				display.VerifyWasCalledInOrder(Once(), inOrder).Flash("Hello", 111)
				// not checking for the 2nd call here
				display.VerifyWasCalledInOrder(Once(), inOrder).Flash("and again", 333)
			}).NotTo(PanicButReport())
		})

		It("fails during InOrder verification when order is not correct", func() {
			Expect(func() {
				inOrder := new(InOrderContext)
				display.VerifyWasCalledInOrder(Once(), inOrder).Flash("again", 222)
				display.VerifyWasCalledInOrder(Once(), inOrder).Flash("Hello", 111)
				display.VerifyWasCalledInOrder(Once(), inOrder).Flash("and again", 333)
			}).To(PanicWith(
				"Expected function call \"Flash\" with params [Hello 111] before function call \"Flash\" with params [again 222]",
			))
		})

	})

	Context("Capturing arguments", func() {
		It("Returns arguments when verifying with argument capture", func() {
			display.Flash("Hello", 111)

			arg1, arg2 := display.VerifyWasCalledOnce().Flash(AnyString(), AnyInt()).getCapturedArguments()

			Expect(arg1).To(Equal("Hello"))
			Expect(arg2).To(Equal(111))
		})

		It("Returns arguments of last invocation when verifying with argument capture", func() {
			display.Flash("Hello", 111)
			display.Flash("Again", 222)

			arg1, arg2 := display.VerifyWasCalled(AtLeast(1)).Flash(AnyString(), AnyInt()).getCapturedArguments()

			Expect(arg1).To(Equal("Again"))
			Expect(arg2).To(Equal(222))
		})

		It("Returns arguments of all invocations when verifying with \"all\" argument capture", func() {
			display.Flash("Hello", 111)
			display.Flash("Again", 222)

			args1, args2 := display.VerifyWasCalled(AtLeast(1)).Flash(AnyString(), AnyInt()).getAllCapturedArguments()

			Expect(args1).To(ConsistOf("Hello", "Again"))
			Expect(args2).To(ConsistOf(111, 222))
		})

		It("Returns *array* arguments of all invocations when verifying with \"all\" argument capture", func() {
			display.ArrayParam([]string{"one", "two"})
			display.ArrayParam([]string{"4", "5", "3"})

			args := display.VerifyWasCalled(AtLeast(1)).ArrayParam(AnyStringSlice()).getAllCapturedArguments()

			Expect(flattenStringSliceOfSlices(args)).To(ConsistOf("one", "two", "3", "4", "5"))
		})

	})

	Describe("Different \"Any\" matcher scenarios", func() {
		It("Succeeds when int-parameter is passed as int but veryfied as float", func() {
			display.FloatParam(1)
			display.VerifyWasCalledOnce().FloatParam(AnyFloat32())
		})

		It("Panics when interface{}-parameter is passed as int, but verified as float", func() {
			Expect(func() {
				display.InterfaceParam(3)
				display.VerifyWasCalledOnce().InterfaceParam(AnyFloat32())
			}).To(PanicWith(
				"Mock invocation count for method \"InterfaceParam\" with params [Any(float32)] " +
					"does not match expectation.\n\n\tExpected: 1; but got: 0",
			))
		})

		It("Panics when interface{}-parameter is passed as float, but verified as int", func() {
			Expect(func() {
				display.InterfaceParam(3.141)
				display.VerifyWasCalledOnce().InterfaceParam(AnyInt())
			}).To(PanicWith(
				"Mock invocation count for method \"InterfaceParam\" with params [Any(int)] " +
					"does not match expectation.\n\n\tExpected: 1; but got: 0",
			))
		})

		It("Succeeds when interface{}-parameter is passed as int and verified as int", func() {
			display.InterfaceParam(3)
			display.VerifyWasCalledOnce().InterfaceParam(AnyInt())
		})

		It("Succeeds when interface{}-parameter is passed as nil and verified as int slice", func() {
			display.InterfaceParam(nil)
			display.VerifyWasCalledOnce().InterfaceParam(AnyIntSlice())
		})

		It("Panics when interface{}-parameter is passed as nil, but verified as int", func() {
			Expect(func() {
				display.InterfaceParam(nil)
				display.VerifyWasCalledOnce().InterfaceParam(AnyInt())
			}).To(PanicWith(
				"Mock invocation count for method \"InterfaceParam\" with params [Any(int)] " +
					"does not match expectation.\n\n\tExpected: 1; but got: 0",
			))
		})

		It("Succeeds when error-parameter is passed as nil and verified as any error", func() {
			display.ErrorParam(nil)
			display.VerifyWasCalledOnce().ErrorParam(AnyError())
		})

		It("Succeeds when error-parameter is passed as string error and verified as any error", func() {
			display.ErrorParam(errors.New("Some error"))
			display.VerifyWasCalledOnce().ErrorParam(AnyError())
		})

		It("Succeeds when http.Request-parameter is passed as null value and verified as any http.Request", func() {
			display.NetHttpRequestParam(http.Request{})
			display.VerifyWasCalledOnce().NetHttpRequestParam(AnyRequest())
		})

		It("Succeeds when http.Request-Pointer-parameter is passed as nil and verified as any *http.Request", func() {
			display.NetHttpRequestPtrParam(nil)
			display.VerifyWasCalledOnce().NetHttpRequestPtrParam(AnyRequestPtr())
		})

		It("Succeeds when http.Request-Pointer-parameter is passed as null value and verified as any *http.Request", func() {
			display.NetHttpRequestPtrParam(&http.Request{})
			display.VerifyWasCalledOnce().NetHttpRequestPtrParam(AnyRequestPtr())
		})

	})

	Describe("Stubbing with multiple ThenReturns versus multiple stubbings with same parameters", func() {
		Context("One stubbing with multiple ThenReturns", func() {
			It("returns the values in the order of the ThenReturns", func() {
				When(display.MultipleParamsAndReturnValue("one", 1)).ThenReturn("first").ThenReturn("second")

				Expect(display.MultipleParamsAndReturnValue("one", 1)).To(Equal("first"))
				Expect(display.MultipleParamsAndReturnValue("one", 1)).To(Equal("second"))
			})
		})

		Context("Multiple stubbings with same parameters", func() {
			It("overrides previous stubbings with last one", func() {
				When(display.MultipleParamsAndReturnValue("one", 1)).ThenReturn("first")
				When(display.MultipleParamsAndReturnValue("one", 1)).ThenReturn("second")

				Expect(display.MultipleParamsAndReturnValue("one", 1)).To(Equal("second"))
				Expect(display.MultipleParamsAndReturnValue("one", 1)).To(Equal("second"))
			})
		})
	})

	Describe("Stubbing methods that have no return value", func() {
		It("Can be stubbed with Panic", func() {
			When(func() { display.Show(AnyString()) }).ThenPanic("bla")
			Expect(func() { display.Show("Hello") }).To(PanicWith("bla"))
		})

		It("Can still work with methods returning a func", func() {
			When(display.FuncReturnValue()).ThenReturn(func() { panic("It's actually a success") })
			Expect(func() { display.FuncReturnValue()() }).To(PanicWith("It's actually a success"))
		})

		FIt("Panics when not using a func with no params", func() {
			Expect(func() {
				When(func(invalid int) { display.Show(AnyString()) })
			}).To(PanicWith("When using 'When' with function that does not return a value, it expects a function with no arguments and no return value."))
		})
	})

})

func flattenStringSliceOfSlices(sliceOfSlices [][]string) (result []string) {
	for _, slice := range sliceOfSlices {
		result = append(result, slice...)
	}
	return
}
