import React, { useState } from 'react';
import { marked } from 'marked';
import { useDropzone } from 'react-dropzone';
import Page from '../../Page';
import PostAPI from '../../Util/PostAPI';
import Go from '../../Go';

import s from './index.less';

type Props = {
    history: Object,
};

const ContextSnippet = ({ context, index }) => {
    const [collapsed, setCollapsed] = useState(true);

    const toggleCollapse = () => {
        setCollapsed(!collapsed);
    };

    const preview =
        context.text.slice(0, 222) + (context.text.length > 222 ? '...' : '');

    return (
        <div className={s.contextSnippet}>
            <h4 onClick={toggleCollapse}>
                Context {index + 1}
                <span className={s.title}>
                    {context.title}
                    <span className={s.arrow}>{collapsed ? '▼' : '▲'}</span>
                </span>
            </h4>
            <div className={collapsed ? s.collapsed : s.expanded}>
                {collapsed ? preview : context.text}
            </div>
        </div>
    );
};

const ResponseDisplay = ({ response }) => {
    if (!response) return null;

    const renderedMarkdown = marked(response.answer);

    return (
        <div className={s.responseDisplay}>
            <div
                className={s.answer}
                dangerouslySetInnerHTML={{ __html: renderedMarkdown }}
            />
            <div className={s.contexts}>
                {response.context.map((context, index) => (
                    <ContextSnippet
                        key={index}
                        context={context}
                        index={index}
                    />
                ))}
            </div>
        </div>
    );
};

const LandingPage = (props: Props): React.Node => {
    const [question, setQuestion] = useState('');
    const [uploadedFiles, setUploadedFiles] = useState([]);
    const [failedFiles, setFailedFiles] = useState({});
    const [errorMessage, setErrorMessage] = useState('');
    const [response, setResponse] = useState(null);
    const [loading, setLoading] = useState(false);

    React.useEffect(() => { }, []);

    const handleAskQuestion = () => {
        // Perform question submission here
        // Set loading state to true
        setLoading(true);
        console.log('Asking question:', question);

        PostAPI.questions
            .submitQuestion(question, 'GPT Turbo')
            .promise.then((response) => {
                console.log('Response:', response);
                setResponse(response);
                setLoading(false);
            })
            .catch((error) => {
                console.log('Error asking question:', error);
                setErrorMessage(
                    error ? JSON.stringify(error) : 'Error asking question'
                );
                setResponse(null);
                setLoading(false);
            });
    };

    React.useEffect(() => {
        const handleKeyDown = (e) => {
            if (e.key === 'Enter' && e.ctrlKey && !loading && question) {
                handleAskQuestion();
            }
        };

        document.addEventListener('keydown', handleKeyDown);

        return () => {
            document.removeEventListener('keydown', handleKeyDown);
        };
    }, [loading, question]);

    const handleFileUpload = (files) => {
        setUploadedFiles([]);
        setFailedFiles({});
        setErrorMessage('');
        setLoading(true);

        PostAPI.upload
            .uploadFiles(files)
            .promise.then((response) => {
                console.log('Response:', response);
                setUploadedFiles(response.successful_file_names ?? []);
                setFailedFiles(response.failed_file_names ?? {});
                setLoading(false);
            })
            .catch((error) => {
                console.log('Error uploading files:', error);
                setErrorMessage(
                    error?.message
                        ? 'Error: ' + error?.message
                        : 'Error uploading files'
                );
                setUploadedFiles([]);
                setFailedFiles({});
                setLoading(false);
            });
    };

    const { getRootProps, getInputProps } = useDropzone({
        onDrop: handleFileUpload,
        multiple: true,
    });

    const failedFilenames = Object.keys(failedFiles);
    let params = (new URL(document.location)).searchParams;
    let userType = params.get('userType'); // "admin"

    return (
        <Page
            pageClass={s.page}
            contentBackgroundClass={s.background}
            contentPreferredWidth={1300}
            contentClass={s.pageContent}>
            <div className={s.text}>
                <h1 style={{fontWeight: 'bold'}}>5g消息AI客服</h1>
                {errorMessage && <div className={s.error}>{errorMessage}</div>}
                <div className={s.workArea}>
                    <div className={s.leftColumn}>
                        {userType === 'admin' && (
                            <div
                                {...getRootProps()}
                                className={[
                                    s.dropzone,
                                    loading ? s.dropzoneLoading : '',
                                ].join(' ')}>
                                <input {...getInputProps()} disabled={loading} />
                                <p>
                                    {loading
                                        ? 'Loading...'
                                        : '拖放文件添加到知识库这里，或点击选择文件'}
                                </p>
                            </div>
                        )}
                         {userType != 'admin' && (<div className={s.questions}>
                            <span>个人gpt账户demo，请节省使用！&#x1F91D;</span>
                            <ul className={s.twoColumnList}>
                              <li>AIM支持手机品牌有哪些？</li>
                              <li>AIM运营商拦截如何解决？</li>
                              <li>AIM模板制作规范有哪些？</li>
                              <li>如何通过H5跳转小程序？</li>
                              <li>富信基本格式规范有哪些？</li>
                              <li>讲讲公司年假制度？</li>
                              <li>AIM非5G消息用户如何接收消息？</li>
                            </ul>
                          </div>
                          )}

                        <div className={s.questionInput}>
                            <textarea
                                value={question}
                                onChange={(e) => setQuestion(e.target.value)}
                                placeholder="Enter your question here..."
                                className={s.textarea}
                            />
                            <button style={{width: 60}}
                                onClick={handleAskQuestion}
                                disabled={loading || !question}
                                className={
                                    loading
                                        ? s.askQuestionDisabled
                                        : s.askQuestion
                                }>
                                提交
                            </button>
                            {loading && <div className={s.loader} />}
                        </div>
                        {response?.tokens && (
                            <div className={s.tokenCount}>
                                TOTAL TOKENS USED:{' '}
                                <span style={{ fontWeight: 900 }}>
                                    {response.tokens}
                                </span>
                            </div>
                        )}
                        {response && <ResponseDisplay response={response} />}
                        <div style={{ height: 32 }} />
                        <div className={s.fileList}>
                            {uploadedFiles.length > 0 && (
                                <div className={s.successfulFiles}>
                                    <h4>Uploaded Files:</h4>
                                    <ul>
                                        {uploadedFiles.map((file, index) => (
                                            <li key={index}>{file}</li>
                                        ))}
                                    </ul>
                                </div>
                            )}
                            {failedFilenames.length > 0 && (
                                <div className={s.failedFiles}>
                                    <h4>Failed Files:</h4>
                                    <ul>
                                        {failedFilenames.map((file, index) => (
                                            <li key={index}>
                                                {file}
                                                <div>
                                                    Reason: {failedFiles[file]}
                                                </div>
                                            </li>
                                        ))}
                                    </ul>
                                </div>
                            )}
                        </div>
                    </div>
                </div>
            </div>
        </Page>
    );
};

export default LandingPage;
