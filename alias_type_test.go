package goldi

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/fgrosse/goldi/tests"
)

var _ = Describe("AliasType", func() {
	It("should implement the TypeFactory interface", func() {
		var factory TypeFactory
		factory = NewAliasType("foo")
		// if this compiles the test passes (next expectation only to make compiler happy)
		Expect(factory).NotTo(BeNil())
	})

	Describe("Arguments()", func() {
		It("should return the aliased service ID", func() {
			typeDef := NewAliasType("foo")
			Expect(typeDef.Arguments()).To(Equal([]interface{}{"@foo"}))
		})
	})

	Describe("Generate()", func() {
		var (
			container *Container
			resolver  *ParameterResolver
		)

		BeforeEach(func() {
			config := map[string]interface{}{}
			container = NewContainer(NewTypeRegistry(), config)
			resolver = NewParameterResolver(container)
		})

		It("should act as alias for the actual type", func() {
			container.Register("foo", NewStructType(tests.MockType{}, "I was created by @foo"))
			alias := NewAliasType("foo")

			generated, err := alias.Generate(resolver)
			Expect(err).NotTo(HaveOccurred())
			Expect(generated).To(BeAssignableToTypeOf(&tests.MockType{}))
			Expect(generated.(*tests.MockType).StringParameter).To(Equal("I was created by @foo"))
		})

		It("should work with func reference types", func() {
			container.Register("foo", NewStructType(tests.MockType{}, "I was created by @foo"))
			alias := NewAliasType("foo::ReturnString")

			generated, err := alias.Generate(resolver)
			Expect(err).NotTo(HaveOccurred())
			Expect(generated).To(BeAssignableToTypeOf(func(string) string { return "" }))
			Expect(generated.(func(string) string)("TEST")).To(Equal("I was created by @foo TEST"))
		})
	})
})
