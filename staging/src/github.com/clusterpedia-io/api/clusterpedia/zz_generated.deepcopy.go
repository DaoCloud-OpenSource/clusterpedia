//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Code generated by deepcopy-gen. DO NOT EDIT.

package clusterpedia

import (
	url "net/url"

	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CollectionResource) DeepCopyInto(out *CollectionResource) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.ResourceTypes != nil {
		in, out := &in.ResourceTypes, &out.ResourceTypes
		*out = make([]CollectionResourceType, len(*in))
		copy(*out, *in)
	}
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]runtime.Object, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				(*out)[i] = (*in)[i].DeepCopyObject()
			}
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CollectionResource.
func (in *CollectionResource) DeepCopy() *CollectionResource {
	if in == nil {
		return nil
	}
	out := new(CollectionResource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CollectionResource) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CollectionResourceList) DeepCopyInto(out *CollectionResourceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]CollectionResource, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CollectionResourceList.
func (in *CollectionResourceList) DeepCopy() *CollectionResourceList {
	if in == nil {
		return nil
	}
	out := new(CollectionResourceList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CollectionResourceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CollectionResourceType) DeepCopyInto(out *CollectionResourceType) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CollectionResourceType.
func (in *CollectionResourceType) DeepCopy() *CollectionResourceType {
	if in == nil {
		return nil
	}
	out := new(CollectionResourceType)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ListOptions) DeepCopyInto(out *ListOptions) {
	*out = *in
	in.ListOptions.DeepCopyInto(&out.ListOptions)
	if in.Names != nil {
		in, out := &in.Names, &out.Names
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ClusterNames != nil {
		in, out := &in.ClusterNames, &out.ClusterNames
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Namespaces != nil {
		in, out := &in.Namespaces, &out.Namespaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.OrderBy != nil {
		in, out := &in.OrderBy, &out.OrderBy
		*out = make([]OrderBy, len(*in))
		copy(*out, *in)
	}
	out.OwnerGroupResource = in.OwnerGroupResource
	if in.Since != nil {
		in, out := &in.Since, &out.Since
		*out = (*in).DeepCopy()
	}
	if in.Before != nil {
		in, out := &in.Before, &out.Before
		*out = (*in).DeepCopy()
	}
	if in.WithContinue != nil {
		in, out := &in.WithContinue, &out.WithContinue
		*out = new(bool)
		**out = **in
	}
	if in.WithRemainingCount != nil {
		in, out := &in.WithRemainingCount, &out.WithRemainingCount
		*out = new(bool)
		**out = **in
	}
	if in.EnhancedFieldSelector != nil {
		out.EnhancedFieldSelector = in.EnhancedFieldSelector.DeepCopySelector()
	}
	if in.ExtraLabelSelector != nil {
		out.ExtraLabelSelector = in.ExtraLabelSelector.DeepCopySelector()
	}
	if in.ExtraQuery != nil {
		in, out := &in.ExtraQuery, &out.ExtraQuery
		*out = make(url.Values, len(*in))
		for key, val := range *in {
			var outVal []string
			if val == nil {
				(*out)[key] = nil
			} else {
				in, out := &val, &outVal
				*out = make([]string, len(*in))
				copy(*out, *in)
			}
			(*out)[key] = outVal
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ListOptions.
func (in *ListOptions) DeepCopy() *ListOptions {
	if in == nil {
		return nil
	}
	out := new(ListOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ListOptions) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OrderBy) DeepCopyInto(out *OrderBy) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OrderBy.
func (in *OrderBy) DeepCopy() *OrderBy {
	if in == nil {
		return nil
	}
	out := new(OrderBy)
	in.DeepCopyInto(out)
	return out
}
