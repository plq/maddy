package msgpipeline

import (
	"errors"
	"testing"

	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/modify"
	"github.com/foxcpp/maddy/internal/testutils"
)

type multipleErrs map[string]error

func (m multipleErrs) SetStatus(rcptTo string, err error) {
	m[rcptTo] = err
}

func TestMsgPipeline_BodyNonAtomic(t *testing.T) {
	err := errors.New("go away")

	target := testutils.Target{
		PartialBodyErr: map[string]error{
			"tester@example.org": err,
		},
	}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	c := multipleErrs{}
	testutils.DoTestDeliveryNonAtomic(t, c, &d, "sender@example.org", []string{"tester@example.org", "tester2@example.org"})

	if c["tester@example.org"] == nil {
		t.Fatalf("no error for tester@example.org")
	}
	if c["tester@example.org"].Error() != err.Error() {
		t.Errorf("wrong error for tester@example.org: %v", err)
	}
}

func TestMsgPipeline_BodyNonAtomic_ModifiedRcpt(t *testing.T) {
	err := errors.New("go away")

	target := testutils.Target{
		PartialBodyErr: map[string]error{
			"tester-alias@example.org": err,
		},
	}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			globalModifiers: modify.Group{
				Modifiers: []module.Modifier{
					testutils.Modifier{
						InstName: "test_modifier",
						RcptTo: map[string]string{
							"tester@example.org": "tester-alias@example.org",
						},
					},
				},
			},
			perSource: map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	c := multipleErrs{}
	testutils.DoTestDeliveryNonAtomic(t, c, &d, "sender@example.org", []string{"tester@example.org"})

	if c["tester@example.org"] == nil {
		t.Fatalf("no error for tester@example.org")
	}
	if c["tester@example.org"].Error() != err.Error() {
		t.Errorf("wrong error for tester@example.org: %v", err)
	}
}

func TestMsgPipeline_BodyNonAtomic_ExpandAtomic(t *testing.T) {
	err := errors.New("go away")

	target, target2 := testutils.Target{
		PartialBodyErr: map[string]error{
			"tester@example.org": err,
		},
	}, testutils.Target{
		BodyErr: err,
	}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			perSource: map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target, &target2},
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	c := multipleErrs{}
	testutils.DoTestDeliveryNonAtomic(t, c, &d, "sender@example.org", []string{"tester@example.org", "tester2@example.org"})

	if c["tester@example.org"] == nil {
		t.Fatalf("no error for tester@example.org")
	}
	if c["tester@example.org"].Error() != err.Error() {
		t.Errorf("wrong error for tester@example.org: %v", err)
	}
	if c["tester2@example.org"] == nil {
		t.Fatalf("no error for tester@example.org")
	}
	if c["tester2@example.org"].Error() != err.Error() {
		t.Errorf("wrong error for tester@example.org: %v", err)
	}
}
