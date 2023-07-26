//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by controller-gen. DO NOT EDIT.

package common

import (
	"k8s.io/api/core/v1"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BaseComponentProbe) DeepCopyInto(out *BaseComponentProbe) {
	*out = *in
	in.BaseComponentProbeHandler.DeepCopyInto(&out.BaseComponentProbeHandler)
	if in.TerminationGracePeriodSeconds != nil {
		in, out := &in.TerminationGracePeriodSeconds, &out.TerminationGracePeriodSeconds
		*out = new(int64)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BaseComponentProbe.
func (in *BaseComponentProbe) DeepCopy() *BaseComponentProbe {
	if in == nil {
		return nil
	}
	out := new(BaseComponentProbe)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BaseComponentProbeHandler) DeepCopyInto(out *BaseComponentProbeHandler) {
	*out = *in
	if in.Exec != nil {
		in, out := &in.Exec, &out.Exec
		*out = new(v1.ExecAction)
		(*in).DeepCopyInto(*out)
	}
	if in.HTTPGet != nil {
		in, out := &in.HTTPGet, &out.HTTPGet
		*out = new(OptionalHTTPGetAction)
		(*in).DeepCopyInto(*out)
	}
	if in.TCPSocket != nil {
		in, out := &in.TCPSocket, &out.TCPSocket
		*out = new(v1.TCPSocketAction)
		**out = **in
	}
	if in.GRPC != nil {
		in, out := &in.GRPC, &out.GRPC
		*out = new(v1.GRPCAction)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BaseComponentProbeHandler.
func (in *BaseComponentProbeHandler) DeepCopy() *BaseComponentProbeHandler {
	if in == nil {
		return nil
	}
	out := new(BaseComponentProbeHandler)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OptionalHTTPGetAction) DeepCopyInto(out *OptionalHTTPGetAction) {
	*out = *in
	out.Port = in.Port
	if in.HTTPHeaders != nil {
		in, out := &in.HTTPHeaders, &out.HTTPHeaders
		*out = make([]v1.HTTPHeader, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OptionalHTTPGetAction.
func (in *OptionalHTTPGetAction) DeepCopy() *OptionalHTTPGetAction {
	if in == nil {
		return nil
	}
	out := new(OptionalHTTPGetAction)
	in.DeepCopyInto(out)
	return out
}